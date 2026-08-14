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
    <div className="mt-6 flex items-center justify-center gap-3 text-sm">
      <button
        type="button"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
        className="rounded border border-border px-3 py-1.5 text-heading disabled:opacity-40"
      >
        Prev
      </button>
      <span className="text-muted">
        Page {page} of {pageCount}
      </span>
      <button
        type="button"
        disabled={page >= pageCount}
        onClick={() => onPageChange(page + 1)}
        className="rounded border border-border px-3 py-1.5 text-heading disabled:opacity-40"
      >
        Next
      </button>
    </div>
  )
}
