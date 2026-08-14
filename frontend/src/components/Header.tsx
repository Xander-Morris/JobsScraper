import { Link } from '@tanstack/react-router'

export function Header() {
  return (
    <header className="flex items-center justify-between border-b border-border px-6 py-4 text-left">
      <Link to="/" className="text-lg font-semibold text-heading no-underline">
        Jobs Scraper
      </Link>
      <nav className="flex gap-5 text-sm">
        <Link
          to="/"
          activeOptions={{ exact: true }}
          className="text-muted no-underline hover:text-heading [&.active]:font-semibold [&.active]:text-heading"
        >
          Jobs
        </Link>
      </nav>
    </header>
  )
}
