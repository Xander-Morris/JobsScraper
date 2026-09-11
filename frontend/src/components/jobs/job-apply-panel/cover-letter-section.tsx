import { CheckIcon, ClipboardIcon, CornerDownLeftIcon, DownloadIcon, SparklesIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { GeneratedContent, Job } from '@/src/api/schemas'
import useCopied from '@/src/hooks/use-copied'
import downloadBlob from '@/src/lib/download-blob'
import slugify from '@/src/lib/slugify'
import { Button } from '../../ui/button'

function BulletCopyButton({ bullet }: { bullet: string }) {
  const { copied, copy } = useCopied()

  return (
    <Button type="button" variant="ghost" size="icon-xs" onClick={() => void copy(bullet)} aria-label="Copy bullet">
      {copied ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />}
    </Button>
  )
}

export default function CoverLetterSection({
  job,
  data,
  onRegenerate,
  isPending
}: {
  job: Job
  data: GeneratedContent
  onRegenerate: () => void
  isPending: boolean
}) {
  const [draft, setDraft] = useState(data.cover_letter)
  const { copied, copy } = useCopied()

  // Regenerating replaces the draft; edits only survive until then.
  useEffect(() => setDraft(data.cover_letter), [data.cover_letter])

  function appendBullet(bullet: string) {
    setDraft((current) => `${current.trimEnd()}\n\n• ${bullet}`)
  }

  return (
    <div className="space-y-3">
      <textarea
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        rows={12}
        aria-label="Cover letter draft"
        className="w-full resize-y rounded-md border border-border bg-background p-2 text-xs leading-relaxed"
      />

      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="xs" onClick={() => void copy(draft)}>
          {copied ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />}{' '}
          {copied ? 'Copied!' : 'Copy letter'}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="xs"
          onClick={() =>
            downloadBlob(
              new Blob([draft], { type: 'text/plain;charset=utf-8' }),
              `${slugify(job.company)}-${slugify(job.title)}-cover-letter.txt`
            )
          }
        >
          <DownloadIcon aria-hidden="true" /> Download .txt
        </Button>
        <Button type="button" variant="ghost" size="xs" onClick={onRegenerate} disabled={isPending}>
          <SparklesIcon aria-hidden="true" /> {isPending ? 'Generating…' : 'Regenerate'}
        </Button>
      </div>

      {data.tailored_bullets.length > 0 && (
        <div className="space-y-1.5">
          <p className="text-[0.7rem] text-muted-foreground">
            Tailored bullets: add one to the letter or paste it into the form
          </p>
          <ul className="space-y-1.5">
            {data.tailored_bullets.map((bullet, index) => (
              <li key={index} className="flex items-start justify-between gap-2 text-xs">
                <span>{bullet}</span>
                <span className="flex shrink-0 gap-0.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => appendBullet(bullet)}
                    aria-label="Add bullet to letter"
                  >
                    <CornerDownLeftIcon aria-hidden="true" />
                  </Button>
                  <BulletCopyButton bullet={bullet} />
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
