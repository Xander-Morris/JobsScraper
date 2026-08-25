import { useId, useState, type FormEvent } from 'react'
import { DownloadIcon, FileTextIcon } from 'lucide-react'
import { downloadResume, useDeleteResumeMutation, useUpdateResumeMutation, useUploadResumeMutation } from '@/src/api/profile'
import type { Resume } from '@/src/api/schemas'
import { Button } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'

const acceptedResumeTypes = '.pdf,.doc,.docx,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document'

export function ResumesSection({ token, resumes }: { token: string; resumes: Resume[] }) {
  const uploadResume = useUploadResumeMutation(token)
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const id = useId()

  function handleUpload(e: FormEvent) {
    e.preventDefault()
    if (!file) return
    setError(null)
    uploadResume.mutate(file, { onSuccess: () => setFile(null), onError: (err) => setError(err instanceof Error ? err.message : 'Unable to upload resume') })
  }

  return <Card>
    <CardHeader>
      <h3 className="text-sm font-semibold text-heading">Resumes</h3>
      <p className="text-xs text-muted-foreground">Upload PDF, DOC, or DOCX files up to 10 MB.</p>
    </CardHeader>
    <CardContent className="space-y-3">
      {resumes.length > 0 ? <ul className="space-y-2">{resumes.map((resume) => <ResumeEntry key={resume.id} token={token} resume={resume} />)}</ul> : <p className="text-sm text-muted-foreground">No resumes uploaded yet.</p>}
      <form onSubmit={handleUpload} className="flex flex-wrap items-center gap-2">
        <Label htmlFor={id} className="sr-only">Resume file</Label>
        <Input id={id} type="file" accept={acceptedResumeTypes} onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="max-w-sm" />
        <Button type="submit" disabled={!file || uploadResume.isPending}>{uploadResume.isPending ? 'Uploading...' : 'Upload resume'}</Button>
      </form>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    </CardContent>
  </Card>
}

function ResumeEntry({ token, resume }: { token: string; resume: Resume }) {
  const updateResume = useUpdateResumeMutation(token)
  const deleteResume = useDeleteResumeMutation(token)
  const [fileName, setFileName] = useState(resume.file_name)
  const [error, setError] = useState<string | null>(null)
  const replaceId = useId()

  async function handleDownload() {
    setError(null)
    try {
      const file = await downloadResume(token, resume.id)
      const url = URL.createObjectURL(file)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = resume.file_name
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to download resume')
    }
  }

  function handleRename(e: FormEvent) {
    e.preventDefault()
    if (!fileName.trim() || fileName === resume.file_name) return
    setError(null)
    updateResume.mutate({ id: resume.id, update: fileName.trim() }, { onError: (err) => setError(err instanceof Error ? err.message : 'Unable to rename resume') })
  }

  function handleReplace(file: File | null) {
    if (!file) return
    setError(null)
    updateResume.mutate({ id: resume.id, update: file }, { onError: (err) => setError(err instanceof Error ? err.message : 'Unable to replace resume') })
  }

  return <li className="rounded-lg border border-border p-3">
    <div className="flex flex-wrap items-center gap-2">
      <FileTextIcon className="size-4 text-muted-foreground" aria-hidden="true" />
      <form onSubmit={handleRename} className="flex flex-1 items-center gap-2">
        <Label htmlFor={`${replaceId}-name`} className="sr-only">Resume name</Label>
        <Input id={`${replaceId}-name`} value={fileName} onChange={(e) => setFileName(e.target.value)} className="max-w-xs" />
        <Button type="submit" variant="outline" size="sm" disabled={updateResume.isPending || fileName === resume.file_name}>Rename</Button>
      </form>
      <span className="text-xs text-muted-foreground">{formatFileSize(resume.file_size)}</span>
      <Button type="button" variant="outline" size="sm" onClick={() => void handleDownload()}><DownloadIcon aria-hidden="true" /> Download</Button>
      <Button type="button" variant="destructive" size="sm" onClick={() => deleteResume.mutate(resume.id)} disabled={deleteResume.isPending}>Delete</Button>
    </div>
    <div className="mt-2 flex items-center gap-2">
      <Label htmlFor={replaceId} className="text-xs text-muted-foreground">Replace file</Label>
      <Input id={replaceId} type="file" accept={acceptedResumeTypes} onChange={(e) => handleReplace(e.target.files?.[0] ?? null)} disabled={updateResume.isPending} className="max-w-sm text-xs" />
    </div>
    {error && <p role="alert" className="mt-2 text-sm text-destructive">{error}</p>}
  </li>
}

function formatFileSize(bytes: number) {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.ceil(bytes / 1024))} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
