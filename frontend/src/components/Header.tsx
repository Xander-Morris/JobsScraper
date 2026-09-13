import { Link } from '@tanstack/react-router'

export function Header() {
  return (
    <header className="flex items-center justify-between border-b border-border px-6 py-4 text-left print:hidden">
      <Link to="/" className="text-lg font-semibold text-heading no-underline">
        Jobs Scraper
      </Link>
      <nav aria-label="Main" className="flex gap-5 text-sm">
        <Link
          to="/profile"
          className="text-muted-foreground no-underline hover:text-heading [&.active]:font-semibold [&.active]:text-heading"
        >
          Profile
        </Link>
        <Link
          to="/jobs"
          activeOptions={{ exact: true }}
          className="text-muted-foreground no-underline hover:text-heading [&.active]:font-semibold [&.active]:text-heading"
        >
          Jobs
        </Link>
      </nav>
    </header>
  )
}
