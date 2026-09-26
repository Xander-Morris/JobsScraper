import { Link } from '@tanstack/react-router'
import { useAuth } from '@/src/stores/auth-store'
import { Button } from './ui/button'

const navLinkClass =
  'rounded-md px-2.5 py-1.5 text-muted-foreground no-underline transition-colors hover:text-heading [&.active]:text-heading'

export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" className={className}>
      <rect width="32" height="32" rx="8" fill="var(--primary)" />
      <rect x="8" y="9" width="16" height="3" rx="1.5" fill="var(--primary-foreground)" />
      <rect x="8" y="14.5" width="11" height="3" rx="1.5" fill="var(--primary-foreground)" />
      <rect x="8" y="20" width="6" height="3" rx="1.5" fill="var(--primary-foreground)" />
    </svg>
  )
}

export function Header() {
  const { isAuthenticated, logout } = useAuth()

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/70 backdrop-blur-md print:hidden">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6 md:grid md:grid-cols-[1fr_auto_1fr]">
        <Link to="/" className="flex items-center gap-2.5 no-underline">
          <Logo className="size-7" />
          <span className="font-semibold tracking-tight text-heading">Jobs Scraper</span>
        </Link>

        <nav aria-label="Main" className="flex items-center gap-1 text-sm">
          <Link to="/jobs" activeOptions={{ includeSearch: false }} className={navLinkClass}>
            Jobs
          </Link>
          <Link to="/profile" className={navLinkClass}>
            Profile
          </Link>
        </nav>

        <div className="hidden justify-end md:flex">
          {isAuthenticated && (
            <Button type="button" variant="ghost" size="sm" onClick={() => logout()}>
              Log out
            </Button>
          )}
        </div>
      </div>
    </header>
  )
}
