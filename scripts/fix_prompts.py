"""Fix remaining prompt files for Faithfulness optimization."""
import os

def fix_generate_go():
    path = r'e:\learngo\eino_agent\internal\pipeline\generate.go'
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    start = content.find('const defaultSystemPrompt')
    end = content.find('// Generate')
    if start < 0 or end < 0:
        print(f'FAILED: generate.go - markers not found (start={start}, end={end})')
        return
    
    new_prompt = '''const defaultSystemPrompt = `你是一个专业的知识库问答助手。

## 铁律：只说上下文里有的（最高优先级）
- 你的每一句回答都**必须**能在上下文信息中找到对应原文。找不到原文支撑的内容**一律不说**。
- **严禁**使用自身训练知识补充、解释、举例或推断。
- 信息不足时直接说明，不要编造任何具体信息。

## 回答格式
1. 每个要点附带 [来源X] 标注
2. 优先使用上下文中的原始措辞`

'''
    content = content[:start] + new_prompt + content[end:]
    with open(path, 'w', encoding='utf-8') as f:
        f.write(content)
    print('SUCCESS: generate.go')

def fix_agentic_rag_go():
    path = r'e:\learngo\eino_agent\internal\pipeline\agentic_rag.go'
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    old = '''systemPrompt = `你是一个专业的知识库问答助手。
回答**必须且只能**基于提供的来源资料，**严禁**使用自身训练知识补充。
如果来源资料不足，请直接说明信息不足。
引用证据时使用 [来源X] 标注。`'''
    
    new = '''systemPrompt = `你是一个专业的知识库问答助手。
你回答中的每一个事实都必须在来源资料中有对应原文。找不到原文支撑的内容一律不说。
严禁使用自身训练知识补充、解释或推断。资料不足时说明信息不足。
引用证据时使用 [来源X] 标注。优先使用资料原文措辞。`'''
    
    if old in content:
        content = content.replace(old, new, 1)
        with open(path, 'w', encoding='utf-8') as f:
            f.write(content)
        print('SUCCESS: agentic_rag.go (fallback prompt)')
    else:
        print('FAILED: agentic_rag.go (fallback prompt) - not found')

def fix_rag_agent_go():
    path = r'e:\learngo\eino_agent\internal\agent\rag_agent.go'
    with open(path, 'r', encoding='utf-8') as f:
        content = f.read()
    
    old = '''SystemPrompt: `你是一个智能助手，可以使用工具来帮助用户解决问题。
当用户询问知识性问题时，优先使用知识库检索工具。
回答**必须且只能**依据检索到的资料，**严禁**使用训练知识补充。资料不足时请明确说明，不要猜测。
当需要实时信息时，使用网络搜索工具。`'''
    
    new = '''SystemPrompt: `你是一个智能助手，可以使用工具来帮助用户解决问题。
当用户询问知识性问题时，优先使用知识库检索工具。
你回答中的每一个事实都必须在检索结果中有对应原文，找不到原文支撑的内容一律不说。严禁使用训练知识补充、解释或推断。资料不足时请明确说明。
当需要实时信息时，使用网络搜索工具。`'''
    
    if old in content:
        content = content.replace(old, new, 1)
        with open(path, 'w', encoding='utf-8') as f:
            f.write(content)
        print('SUCCESS: rag_agent.go')
    else:
        print('FAILED: rag_agent.go - not found')
        # Debug
        idx = content.find('SystemPrompt: `')
        if idx >= 0:
            print(f'  Found SystemPrompt at {idx}: {repr(content[idx:idx+200])}')

if __name__ == '__main__':
    fix_generate_go()
    fix_agentic_rag_go()
    fix_rag_agent_go()
