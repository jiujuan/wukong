import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'

type MarkdownViewProps = {
  content: string
}

export function MarkdownView({ content }: MarkdownViewProps) {
  return (
    <div className="max-w-none space-y-3 text-sm leading-6 text-zinc-800 [&_code]:rounded [&_code]:bg-zinc-200 [&_code]:px-1 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold [&_li]:list-disc [&_li]:ml-5 [&_pre]:overflow-auto [&_pre]:rounded-md [&_pre]:border [&_pre]:border-zinc-300 [&_pre]:bg-zinc-100 [&_pre]:p-3 [&_table]:w-full [&_td]:border [&_td]:border-zinc-300 [&_td]:p-2 [&_th]:border [&_th]:border-zinc-300 [&_th]:bg-zinc-100 [&_th]:p-2">
      <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeHighlight]}>
        {content}
      </ReactMarkdown>
    </div>
  )
}
