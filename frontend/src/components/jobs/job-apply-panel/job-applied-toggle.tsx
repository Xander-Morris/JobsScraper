import { useMarkJobAppliedMutation, useUnmarkJobAppliedMutation } from '@/src/api/jobs';
import type { Job } from '@/src/api/schemas';
import { CircleCheckIcon } from 'lucide-react';
import { Button } from '../../ui/button';

export default function JobAppliedToggle({ token, job }: { token: string; job: Job }) {
  const markApplied = useMarkJobAppliedMutation(token)
  const unmarkApplied = useUnmarkJobAppliedMutation(token)

  return job.applied ? (
    <Button
      type="button"
      variant="secondary"
      onClick={() => unmarkApplied.mutate(job.id)}
      disabled={unmarkApplied.isPending}
    >
      <CircleCheckIcon aria-hidden="true" /> Remove from applied
    </Button>
  ) : (
    <Button type="button" variant="outline" onClick={() => markApplied.mutate(job.id)} disabled={markApplied.isPending}>
      Mark as applied
    </Button>
  )
}
