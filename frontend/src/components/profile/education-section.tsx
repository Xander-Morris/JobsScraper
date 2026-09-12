import { useId, useState, type FormEvent } from 'react'
import { useAddEducationMutation, useDeleteEducationMutation } from '@/src/api/profile'
import type { Education } from '@/src/api/schemas'
import { Button } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { useAuth } from '@/src/stores/profile-store'

export function EducationSection({ education }: { education: Education[] }) {
  const { token } = useAuth()
  const addEducation = useAddEducationMutation(token)
  const deleteEducation = useDeleteEducationMutation(token)
  const [schoolName, setSchoolName] = useState('')
  const [major, setMajor] = useState('')
  const [degree, setDegree] = useState('')
  const [gpa, setGpa] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    addEducation.mutate(
      {
        school_name: schoolName,
        major,
        degree,
        gpa: gpa ? Number(gpa) : null,
        start_date: startDate || undefined,
        end_date: endDate || undefined
      },
      {
        onSuccess: () => {
          setSchoolName('')
          setMajor('')
          setDegree('')
          setGpa('')
          setStartDate('')
          setEndDate('')
        }
      }
    )
  }

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Education</h3>
      </CardHeader>
      <CardContent>
        {education.length > 0 && (
          <ul className="mb-3 space-y-1">
            {education.map((entry) => (
              <li key={entry.id} className="flex items-center justify-between text-sm">
                <span>
                  {entry.school_name} · {entry.major}, {entry.degree}
                  {entry.gpa != null ? ` · GPA ${entry.gpa}` : ''}
                  {entry.start_date ? ` · ${entry.start_date} – ${entry.end_date ?? 'present'}` : ''}
                </span>
                <Button type="button" variant="ghost" size="sm" onClick={() => deleteEducation.mutate(entry.id)}>
                  Remove
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={handleSubmit} className="flex flex-wrap items-end gap-2">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-school`} className="sr-only">
              School
            </Label>
            <Input
              id={`${id}-school`}
              required
              placeholder="School"
              value={schoolName}
              onChange={(e) => setSchoolName(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-major`} className="sr-only">
              Major
            </Label>
            <Input
              id={`${id}-major`}
              required
              placeholder="Major"
              value={major}
              onChange={(e) => setMajor(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-degree`} className="sr-only">
              Degree
            </Label>
            <Input
              id={`${id}-degree`}
              required
              placeholder="Degree"
              value={degree}
              onChange={(e) => setDegree(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-gpa`} className="sr-only">
              GPA
            </Label>
            <Input
              id={`${id}-gpa`}
              type="number"
              step="0.01"
              min="0"
              max="4"
              placeholder="GPA"
              className="w-20"
              value={gpa}
              onChange={(e) => setGpa(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-start`}>Start date</Label>
            <Input id={`${id}-start`} type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-end`}>End date</Label>
            <Input id={`${id}-end`} type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </div>
          <Button type="submit" disabled={addEducation.isPending}>
            Add
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
