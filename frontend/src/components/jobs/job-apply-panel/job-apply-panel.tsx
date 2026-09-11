import { useGenerateApplicationContentMutation } from '@/src/api/jobs'
import { useDownloadResumeMutation, useProfileQuery, useResumeExtractionQuery } from '@/src/api/profile'
import type { Job } from '@/src/api/schemas'
import { matchSkills } from '@/src/lib/skills'
import { SparklesIcon } from 'lucide-react'
import { Button } from '../../ui/button'
import { Card, CardContent, CardHeader } from '../../ui/card'
import ContactSection, { type ContactField } from './contact-section'
import CoverLetterSection from './cover-letter-section'
import FitSection from './fit-section'
import OpenAndToggle from './open-and-toggle'
import ResumeSection from './resume-section'
import SectionHeading from './section-heading'

export default function JobApplyPanel({ token, job }: { token: string; job: Job }) {
  const { data: profile, isLoading: profileLoading } = useProfileQuery(token)
  const activeResume = profile?.resumes?.find((resume) => resume.is_active) ?? null
  const { data: extraction } = useResumeExtractionQuery(token, activeResume?.id ?? 0, {
    enabled: activeResume != null
  })
  const downloadResume = useDownloadResumeMutation(token)
  const generateContent = useGenerateApplicationContentMutation(token)
  const downloadError = downloadResume.error instanceof Error ? downloadResume.error.message : null
  const generateError = generateContent.error instanceof Error ? generateContent.error.message : null
  const resumeReady = extraction?.status === 'completed'
  const { matched, missing } = matchSkills(job.tags, job.description, extraction?.skills)

  if (profileLoading || !profile) return null

  const fields: ContactField[] = [
    { label: 'Name', value: profile.name },
    { label: 'Email', value: profile.email },
    { label: 'Phone', value: extraction?.phone },
    { label: 'LinkedIn', value: profile.linked_in },
    { label: 'GitHub', value: profile.github },
    { label: 'Portfolio', value: profile.portfolio }
  ].filter((field): field is ContactField => !!field.value)

  return (
    <Card>
      <CardHeader>
        <h2 className="text-sm font-semibold text-heading">Apply assist</h2>
        <p className="text-xs text-muted-foreground">Everything {job.company}&rsquo;s form asks for, one click away.</p>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <OpenAndToggle job={job}/>

        {fields.length > 0 ? (
          <ContactSection fields={fields} />
        ) : (
          <p className="text-xs text-muted-foreground">Fill in your profile to use apply assist.</p>
        )}

        <ResumeSection activeResume={activeResume} downloadResume={downloadResume} />
        <FitSection matched={matched} missing={missing} />

        <section className="space-y-2 border-t border-border pt-3">
          <SectionHeading step={3} title="Cover letter" />
          {!resumeReady && (
            <p className="text-xs text-muted-foreground">
              {activeResume
                ? 'Waiting on resume parsing before a letter can be drafted.'
                : 'Upload and activate a resume to draft a letter for this role.'}
            </p>
          )}
          {resumeReady && !generateContent.data && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => generateContent.mutate(job.id)}
              disabled={generateContent.isPending}
            >
              <SparklesIcon aria-hidden="true" />
              {generateContent.isPending ? 'Generating…' : 'Draft letter for this role'}
            </Button>
          )}
          {generateContent.data && (
            <CoverLetterSection
              job={job}
              data={generateContent.data}
              onRegenerate={() => generateContent.mutate(job.id)}
              isPending={generateContent.isPending}
            />
          )}
        </section>

        {downloadError && (
          <p role="alert" className="text-xs text-destructive">
            {downloadError}
          </p>
        )}

        {generateError && (
          <p role="alert" className="text-xs text-destructive">
            {generateError}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
