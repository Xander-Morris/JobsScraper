import { DownloadIcon } from 'lucide-react'
import type { useDownloadResumeMutation } from '@/src/api/profile'
import type { Resume } from '@/src/api/schemas'
import downloadBlob from '@/src/lib/download-blob'
import { Button } from '../../ui/button'
import SectionHeading from './section-heading'

export default function ResumeSection({
  activeResume,
  downloadResume
}: {
  activeResume: Resume | null
  downloadResume: ReturnType<typeof useDownloadResumeMutation>
}) {
  function handleDownload() {
    if (!activeResume) return
    downloadResume.mutate(activeResume.id, {
      onSuccess: (file) => downloadBlob(file, activeResume.file_name)
    })
  }

  return (
    <section className="space-y-1.5 border-t border-border pt-3">
      <SectionHeading step={2} title="Resume" />
      {activeResume ? (
        <div className="flex items-center justify-between gap-2">
          <span className="min-w-0 truncate text-xs text-muted-foreground">{activeResume.file_name}</span>
          <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={handleDownload}
            disabled={downloadResume.isPending}
          >
            <DownloadIcon aria-hidden="true" /> Download
          </Button>
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">No active resume. Set one in your profile to attach it here.</p>
      )}
    </section>
  )
}
