import type {
  Job,
  JobListItem,
  Profile,
  Resume,
  ResumeExtraction,
  TailoredResume,
  TailoredResumeContent,
  TriviaQuiz,
  TriviaQuizSummary
} from '@/src/api/schemas'

// Builders return wire-shaped objects that pass api/schemas.ts, with overrides.

export function makeJob(overrides: Partial<Job> = {}): Job {
  return {
    id: 1,
    title: 'Backend Engineer',
    company: 'Acme',
    location: 'Remote',
    workplace_type: 'remote',
    tags: ['go', 'postgres'],
    salary_min: 120000,
    salary_max: 150000,
    posted_at: new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString(),
    url: 'https://acme.example/jobs/1',
    description: 'Build Go services on Postgres.',
    match_score: null,
    match_percent: null,
    applied: false,
    ...overrides
  }
}

export function makeJobListItem(overrides: Partial<JobListItem> = {}): JobListItem {
  const { description: _description, ...item } = makeJob(overrides as Partial<Job>)
  void _description
  return item
}

export function makeResume(overrides: Partial<Resume> = {}): Resume {
  return {
    id: 10,
    file_name: 'resume.pdf',
    content_type: 'application/pdf',
    file_size: 2048,
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides
  }
}

export function makeProfile(overrides: Partial<Profile> = {}): Profile {
  return {
    id: 1,
    email: 'ada@example.com',
    name: '',
    address: '',
    linked_in: '',
    github: '',
    portfolio: '',
    email_notifications: false,
    education: [],
    skills: [],
    work_experience: [],
    resumes: [],
    ...overrides
  }
}

export function makeExtraction(overrides: Partial<ResumeExtraction> = {}): ResumeExtraction {
  return {
    resume_id: 10,
    status: 'completed',
    full_name: 'Ada Lovelace',
    email: 'ada@example.com',
    phone: '555-0100',
    linked_in: 'https://linkedin.com/in/ada',
    github: 'https://github.com/ada',
    portfolio: 'https://ada.dev',
    summary: 'Engineer',
    skills: ['Go', 'Postgres'],
    education: [],
    work_experience: [],
    projects: [],
    error: '',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides
  }
}

export function makeTailoredContent(overrides: Partial<TailoredResumeContent> = {}): TailoredResumeContent {
  return {
    full_name: 'Ada Lovelace',
    email: 'ada@example.com',
    phone: '555-0100',
    linked_in: '',
    github: '',
    portfolio: '',
    summary: 'Go engineer',
    skills: ['Go'],
    education: [],
    work_experience: [
      {
        company: 'Acme',
        job_title: 'Engineer',
        location: 'Remote',
        start_date: '2022-01-01',
        end_date: '',
        bullets: [
          { text: 'Built APIs', source: 'W0.B0' },
          { text: 'Ran on-call', source: 'W0.B1' }
        ]
      }
    ],
    projects: [],
    ...overrides
  }
}

export function makeTailoredResume(overrides: Partial<TailoredResume> = {}): TailoredResume {
  return {
    job_id: 1,
    content: makeTailoredContent(),
    stale: false,
    updated_at: '2026-01-02T00:00:00Z',
    ...overrides
  }
}

export function makeQuiz(overrides: Partial<TriviaQuiz> = {}): TriviaQuiz {
  return {
    id: 5,
    source: 'prompt',
    topic: 'Go',
    difficulty: 'easy',
    score: 0,
    completed_at: null,
    created_at: '2026-01-01T00:00:00Z',
    questions: [
      {
        question: 'What is a goroutine?',
        options: ['A thread', 'A lightweight thread', 'A process', 'A lock'],
        correct_index: 1,
        explanation: 'Managed by the Go runtime.',
        topic: 'Go'
      },
      {
        question: 'What does defer do?',
        options: ['Runs later', 'Runs now', 'Panics', 'Nothing'],
        correct_index: 0,
        explanation: 'Runs when the function returns.',
        topic: 'Go'
      }
    ],
    answers: [],
    ...overrides
  }
}

export function makeQuizSummary(overrides: Partial<TriviaQuizSummary> = {}): TriviaQuizSummary {
  return {
    id: 5,
    source: 'prompt',
    topic: 'Go',
    difficulty: 'easy',
    score: 0,
    completed_at: null,
    created_at: '2026-01-01T00:00:00Z',
    question_count: 2,
    answered_count: 0,
    ...overrides
  }
}
