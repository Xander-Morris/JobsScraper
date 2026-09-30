import { createFileRoute } from '@tanstack/react-router'
import TriviaGenerator from '@/src/components/trivia/trivia-generator'
import TriviaHistory from '@/src/components/trivia/trivia-history'
import { profileQueryOptions } from '@/src/hooks/use-profile'

export const Route = createFileRoute('/trivia/')({
  loader: ({ context: { auth, queryClient } }) => {
    if (auth.isAuthenticated) void queryClient.prefetchQuery(profileQueryOptions)
  },
  component: TriviaPage
})

function TriviaPage() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-medium tracking-[-0.03em]">Trivia</h1>
        <p className="mt-1 text-sm text-muted-foreground">Sharpen up before the interview.</p>
      </div>
      <TriviaGenerator />
      <TriviaHistory />
    </div>
  )
}
