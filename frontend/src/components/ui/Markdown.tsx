import ReactMarkdown from 'react-markdown'

export function Markdown({ children }: { children: string }) {
  return <div className="markdown"><ReactMarkdown
    skipHtml
    disallowedElements={['img']}
    components={{
      a: ({ href, children }) => href
        ? <a href={href} target="_blank" rel="noopener noreferrer">{children}</a>
        : <span>{children}</span>,
    }}
  >{children}</ReactMarkdown></div>
}
