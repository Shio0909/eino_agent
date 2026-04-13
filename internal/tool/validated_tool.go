package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ValidatedTool 为工具添加基于 schema 的参数自动校验。
// 在 InvokableRun 前校验 JSON 格式和必填字段，返回清晰的错误提示帮助 LLM 自我修正。
type ValidatedTool struct {
	inner    tool.InvokableTool
	toolInfo *schema.ToolInfo // 缓存
}

// WrapWithValidation 包装工具，添加参数自动校验
func WrapWithValidation(t tool.InvokableTool) tool.InvokableTool {
	return &ValidatedTool{inner: t}
}

func (v *ValidatedTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	if v.toolInfo != nil {
		return v.toolInfo, nil
	}
	info, err := v.inner.Info(ctx)
	if err != nil {
		return nil, err
	}
	v.toolInfo = info
	return info, nil
}

func (v *ValidatedTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	info, err := v.Info(ctx)
	if err != nil {
		return v.inner.InvokableRun(ctx, input, opts...)
	}

	if err := validateInput(info, input); err != nil {
		return "", fmt.Errorf("参数校验失败: %w", err)
	}

	return v.inner.InvokableRun(ctx, input, opts...)
}

func validateInput(info *schema.ToolInfo, input string) error {
	if info == nil || info.ParamsOneOf == nil {
		return nil
	}

	// 通过 JSON Schema 获取参数定义
	jsonSchema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil || jsonSchema == nil {
		return nil
	}

	// 解析输入 JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		// 构建友好的错误信息
		var paramHints []string
		if jsonSchema.Properties != nil {
			for pair := jsonSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
				name := pair.Key
				prop := pair.Value
				required := isRequired(name, jsonSchema.Required)
				marker := ""
				if required {
					marker = " (必填)"
				}
				paramHints = append(paramHints, fmt.Sprintf("%s: %s%s", name, prop.Type, marker))
			}
		}
		return fmt.Errorf("JSON 格式无效，期望参数: {%s}，收到: %s",
			strings.Join(paramHints, ", "), truncate(input, 100))
	}

	// 检查必填字段
	var missing []string
	for _, reqField := range jsonSchema.Required {
		val, exists := parsed[reqField]
		if !exists || val == nil {
			missing = append(missing, reqField)
			continue
		}
		// 字符串类型不能为空
		if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
			missing = append(missing, reqField+"(不能为空字符串)")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("缺少必填参数: %s", strings.Join(missing, ", "))
	}

	// 检查类型匹配
	if jsonSchema.Properties != nil {
		var typeErrors []string
		for pair := jsonSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
			name := pair.Key
			prop := pair.Value
			val, exists := parsed[name]
			if !exists || val == nil {
				continue
			}
			if errMsg := checkType(name, prop.Type, val); errMsg != "" {
				typeErrors = append(typeErrors, errMsg)
			}
			// enum 检查
			if len(prop.Enum) > 0 {
				if s, ok := val.(string); ok {
					found := false
					enumStrs := make([]string, 0, len(prop.Enum))
					for _, e := range prop.Enum {
						es := fmt.Sprintf("%v", e)
						enumStrs = append(enumStrs, es)
						if s == es {
							found = true
						}
					}
					if !found {
						typeErrors = append(typeErrors, fmt.Sprintf(
							"%s 的值 %q 不在允许范围 [%s] 内", name, s, strings.Join(enumStrs, ", ")))
					}
				}
			}
		}
		if len(typeErrors) > 0 {
			return fmt.Errorf("参数错误: %s", strings.Join(typeErrors, "; "))
		}
	}

	return nil
}

func checkType(name, schemaType string, val interface{}) string {
	switch schemaType {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Sprintf("%s 应为 string，实际为 %T", name, val)
		}
	case "integer":
		switch v := val.(type) {
		case float64:
			if v != float64(int64(v)) {
				return fmt.Sprintf("%s 应为整数，实际为 %v", name, v)
			}
		default:
			return fmt.Sprintf("%s 应为 integer，实际为 %T", name, val)
		}
	case "number":
		if _, ok := val.(float64); !ok {
			return fmt.Sprintf("%s 应为 number，实际为 %T", name, val)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return fmt.Sprintf("%s 应为 boolean，实际为 %T", name, val)
		}
	case "array":
		if _, ok := val.([]interface{}); !ok {
			return fmt.Sprintf("%s 应为 array，实际为 %T", name, val)
		}
	}
	return ""
}

func isRequired(name string, required []string) bool {
	for _, r := range required {
		if r == name {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Ensure interface implementation
var _ tool.InvokableTool = (*ValidatedTool)(nil)
