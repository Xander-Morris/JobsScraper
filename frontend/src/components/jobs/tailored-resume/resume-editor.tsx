import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon, XIcon } from 'lucide-react'
import type { ResumeExtraction, TailoredBullet, TailoredResumeContent } from '@/src/api/schemas'
import { badgeVariants } from '@/src/components/ui/badge'
import { Button } from '@/src/components/ui/button'
import { cn } from '@/src/lib/utils'

type BulletSection = 'work_experience' | 'projects'

const textareaClass = 'w-full resize-y rounded-md border border-border bg-background p-2 text-xs leading-relaxed'

function move<T>(items: T[], from: number, to: number): T[] {
  if (to < 0 || to >= items.length) return items

  const next = [...items]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item)
  return next
}

function withBullets(
  content: TailoredResumeContent,
  section: BulletSection,
  index: number,
  update: (bullets: TailoredBullet[]) => TailoredBullet[]
): TailoredResumeContent {
  if (section === 'work_experience') {
    return {
      ...content,
      work_experience: content.work_experience.map((entry, i) =>
        i === index ? { ...entry, bullets: update(entry.bullets) } : entry
      )
    }
  }

  return {
    ...content,
    projects: content.projects.map((entry, i) => (i === index ? { ...entry, bullets: update(entry.bullets) } : entry))
  }
}

function originalBullet(extraction: ResumeExtraction | undefined, source: string): string | undefined {
  const match = /^([WP])(\d+)\.B(\d+)$/.exec(source)
  if (!extraction || !match) return undefined

  const sections = match[1] === 'W' ? extraction.work_experience : extraction.projects
  return sections?.[Number(match[2])]?.bullets?.[Number(match[3])]
}

function BulletList({
  content,
  section,
  index,
  onChange,
  extraction
}: {
  content: TailoredResumeContent
  section: BulletSection
  index: number
  onChange: (content: TailoredResumeContent) => void
  extraction?: ResumeExtraction
}) {
  const bullets = content[section][index].bullets
  const edit = (update: (bullets: TailoredBullet[]) => TailoredBullet[]) =>
    onChange(withBullets(content, section, index, update))

  return (
    <div className="space-y-2">
      {bullets.map((bullet, bi) => {
        const original = originalBullet(extraction, bullet.source)

        return (
          <div key={bi} className="space-y-1">
            <div className="flex items-start gap-1">
              <textarea
                value={bullet.text}
                rows={2}
                aria-label={`Bullet ${bi + 1}`}
                className={textareaClass}
                onChange={(event) =>
                  edit((list) => list.map((b, i) => (i === bi ? { ...b, text: event.target.value } : b)))
                }
              />
              <div className="flex shrink-0 flex-col">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  aria-label="Move bullet up"
                  disabled={bi === 0}
                  onClick={() => edit((list) => move(list, bi, bi - 1))}
                >
                  <ArrowUpIcon aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  aria-label="Move bullet down"
                  disabled={bi === bullets.length - 1}
                  onClick={() => edit((list) => move(list, bi, bi + 1))}
                >
                  <ArrowDownIcon aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-xs"
                  aria-label="Remove bullet"
                  onClick={() => edit((list) => list.filter((_, i) => i !== bi))}
                >
                  <XIcon aria-hidden="true" />
                </Button>
              </div>
            </div>
            {original && original !== bullet.text && (
              <p className="text-[0.7rem] text-muted-foreground">Original: {original}</p>
            )}
          </div>
        )
      })}

      <Button
        type="button"
        variant="ghost"
        size="xs"
        onClick={() => edit((list) => [...list, { text: '', source: '' }])}
      >
        <PlusIcon aria-hidden="true" /> Add bullet
      </Button>
    </div>
  )
}

export default function ResumeEditor({
  content,
  onChange,
  extraction
}: {
  content: TailoredResumeContent
  onChange: (content: TailoredResumeContent) => void
  extraction?: ResumeExtraction
}) {
  return (
    <div className="space-y-6 text-sm">
      <section className="space-y-1.5">
        <label htmlFor="tailored-summary" className="text-xs font-medium text-heading">
          Summary
        </label>
        <textarea
          id="tailored-summary"
          rows={4}
          value={content.summary}
          onChange={(event) => onChange({ ...content, summary: event.target.value })}
          className={textareaClass}
        />
      </section>

      {content.skills.length > 0 && (
        <section className="space-y-1.5">
          <p className="text-xs font-medium text-heading">Skills</p>
          <ul className="flex flex-wrap gap-1.5">
            {content.skills.map((skill, index) => (
              <li key={skill}>
                <button
                  type="button"
                  onClick={() => onChange({ ...content, skills: content.skills.filter((_, i) => i !== index) })}
                  aria-label={`Remove ${skill}`}
                  className={cn(
                    badgeVariants({ variant: 'secondary' }),
                    'gap-1 hover:bg-destructive/10 hover:text-destructive'
                  )}
                >
                  {skill}
                  <XIcon aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}

      {content.work_experience.length > 0 && (
        <section className="space-y-4">
          <p className="text-xs font-medium text-heading">Experience</p>
          {content.work_experience.map((role, index) => (
            <div key={index} className="space-y-2 border-t border-border pt-3">
              <p className="text-xs font-medium text-heading">
                {role.job_title} · {role.company}
              </p>
              <BulletList
                content={content}
                section="work_experience"
                index={index}
                onChange={onChange}
                extraction={extraction}
              />
            </div>
          ))}
        </section>
      )}

      {content.projects.length > 0 && (
        <section className="space-y-4">
          <p className="text-xs font-medium text-heading">Projects</p>
          {content.projects.map((project, index) => (
            <div key={index} className="space-y-2 border-t border-border pt-3">
              <div className="flex items-center justify-between gap-2">
                <p className="text-xs font-medium text-heading">{project.name}</p>
                <Button
                  type="button"
                  variant="ghost"
                  size="xs"
                  onClick={() => onChange({ ...content, projects: content.projects.filter((_, i) => i !== index) })}
                >
                  <Trash2Icon aria-hidden="true" /> Remove
                </Button>
              </div>
              <BulletList
                content={content}
                section="projects"
                index={index}
                onChange={onChange}
                extraction={extraction}
              />
            </div>
          ))}
        </section>
      )}
    </div>
  )
}
