import type { ReactNode } from 'react'
import type { TailoredBullet, TailoredResumeContent } from '@/src/api/schemas'

function formatMonth(date: string): string {
  const match = /^(\d{4})-(\d{2})/.exec(date)
  if (!match) return date

  return new Date(Number(match[1]), Number(match[2]) - 1).toLocaleString('en-US', { month: 'short', year: 'numeric' })
}

function formatRange(start: string, end: string): string {
  if (!start && !end) return ''

  return `${start ? formatMonth(start) : ''} – ${end ? formatMonth(end) : 'Present'}`
}

function ResumeSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mt-4">
      <p className="border-b border-black pb-0.5 text-xs font-bold tracking-wider uppercase">{title}</p>
      <div className="mt-1.5 space-y-2">{children}</div>
    </section>
  )
}

function EntryHeader({ title, dates }: { title: string; dates: string }) {
  return (
    <div className="flex justify-between gap-4">
      <p className="font-semibold">{title}</p>
      {dates && <p className="shrink-0">{dates}</p>}
    </div>
  )
}

function Bullets({ bullets }: { bullets: TailoredBullet[] }) {
  const items = bullets.filter((bullet) => bullet.text.trim())
  if (items.length === 0) return null

  return (
    <ul className="mt-1 list-disc space-y-0.5 pl-5">
      {items.map((bullet, index) => (
        <li key={index}>{bullet.text}</li>
      ))}
    </ul>
  )
}

export default function ResumePreview({ content }: { content: TailoredResumeContent }) {
  const contact = [content.email, content.phone, content.linked_in, content.github, content.portfolio].filter(Boolean)

  return (
    <article
      aria-label="Resume preview"
      className="rounded-md border border-border bg-white p-8 text-left font-serif text-[13px] leading-snug text-black print:rounded-none print:border-0 print:p-0"
    >
      <header className="text-center">
        <p className="text-2xl font-semibold">{content.full_name}</p>
        {contact.length > 0 && <p className="mt-1 text-xs">{contact.join(' | ')}</p>}
      </header>

      {content.summary.trim() && (
        <ResumeSection title="Summary">
          <p>{content.summary}</p>
        </ResumeSection>
      )}

      {content.skills.length > 0 && (
        <ResumeSection title="Skills">
          <p>{content.skills.join(', ')}</p>
        </ResumeSection>
      )}

      {content.work_experience.length > 0 && (
        <ResumeSection title="Experience">
          {content.work_experience.map((role, index) => (
            <div key={index} className="break-inside-avoid">
              <EntryHeader
                title={[role.job_title, role.company].filter(Boolean).join(', ')}
                dates={formatRange(role.start_date, role.end_date)}
              />
              {role.location && <p className="italic">{role.location}</p>}
              <Bullets bullets={role.bullets} />
            </div>
          ))}
        </ResumeSection>
      )}

      {content.projects.length > 0 && (
        <ResumeSection title="Projects">
          {content.projects.map((project, index) => (
            <div key={index} className="break-inside-avoid">
              <EntryHeader
                title={project.technologies.length > 0 ? `${project.name} (${project.technologies.join(', ')})` : project.name}
                dates=""
              />
              {project.url && <p>{project.url}</p>}
              <Bullets bullets={project.bullets} />
            </div>
          ))}
        </ResumeSection>
      )}

      {content.education.length > 0 && (
        <ResumeSection title="Education">
          {content.education.map((edu, index) => (
            <div key={index} className="break-inside-avoid">
              <EntryHeader
                title={[[edu.degree, edu.major].filter(Boolean).join(' in '), edu.school_name].filter(Boolean).join(', ')}
                dates={formatRange(edu.start_date, edu.end_date)}
              />
            </div>
          ))}
        </ResumeSection>
      )}
    </article>
  )
}
