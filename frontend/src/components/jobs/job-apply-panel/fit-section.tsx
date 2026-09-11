import { Badge } from '../../ui/badge'

// What to lead with, and what the application will probe that the resume doesn't cover.
export default function FitSection({ matched, missing }: { matched: string[]; missing: string[] }) {
  if (matched.length === 0 && missing.length === 0) return null

  return (
    <section className="space-y-2 border-t border-border pt-3">
      <p className="text-xs font-medium text-heading">Fit check</p>

      {matched.length > 0 && (
        <div className="space-y-1">
          <p className="text-[0.7rem] text-muted-foreground">On your resume, lead with these</p>
          <div className="flex flex-wrap gap-1">
            {matched.slice(0, 12).map((skill) => (
              <Badge key={skill} variant="secondary">
                {skill}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {missing.length > 0 && (
        <div className="space-y-1">
          <p className="text-[0.7rem] text-muted-foreground">Asked for, not on your resume</p>
          <div className="flex flex-wrap gap-1">
            {missing.slice(0, 12).map((skill) => (
              <Badge key={skill} variant="outline">
                {skill}
              </Badge>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}
