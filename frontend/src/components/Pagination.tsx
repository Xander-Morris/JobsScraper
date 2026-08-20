import { Button } from './ui/button'

export function Pagination({
  page,
  total,
  pageSize,
  onPageChange,
}: {
  page: number
  total: number
  pageSize: number
  onPageChange: (page: number) => void
}) {
  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  if (pageCount <= 1) return null

  return (
    <nav aria-label="Pagination" className="mt-6 flex items-center justify-center gap-3 text-sm">
      <Button type="button" variant="outline" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
        Prev
      </Button>
      <span aria-live="polite" className="text-muted-foreground">
        Page {page} of {pageCount}
      </span>
      <Button type="button" variant="outline" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)}>
        Next
      </Button>
    </nav>
  )
}
