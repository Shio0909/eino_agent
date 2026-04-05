import { AlertTriangle } from 'lucide-react'

const WEB_SEARCH_MARKER = '此信息来源于网络搜索'

interface Props {
  content: string
}

export default function WebSearchWarning({ content }: Props) {
  if (!content || !content.includes(WEB_SEARCH_MARKER)) {
    return null
  }

  return (
    <div className="flex items-start gap-2 px-3 py-2.5 mb-3 rounded-lg border border-red-300 bg-red-50 dark:border-red-700 dark:bg-red-950/40 text-red-700 dark:text-red-300 text-sm">
      <AlertTriangle size={16} className="shrink-0 mt-0.5" />
      <span>
        本回答部分内容来源于<strong>网络搜索</strong>，准确性无法保证，请自行验证关键信息。
      </span>
    </div>
  )
}
