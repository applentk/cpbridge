import type {
  User,
  Problem,
  ProblemStatement,
  ProblemSet,
  Contest,
  Standings,
  Submission,
} from '@cpbridge/contracts';

export const mockRegularUser: User = {
  id: 'usr_reg_123',
  email: 'coder@example.com',
  username: 'tourist_fan',
  role: 'USER',
  isActive: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

export const mockAdminUser: User = {
  id: 'usr_adm_999',
  email: 'admin@cpbridge.dev',
  username: 'root_admin',
  role: 'ADMIN',
  isActive: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
};

export const mockProblems: Problem[] = [
  {
    id: 'prb_cf_1000A',
    platform: 'CODEFORCES',
    externalId: '1000A',
    title: 'Codehorses T-shirts',
    url: 'https://codeforces.com/problemset/problem/1000/A',
    difficulty: 1400,
    tags: ['greedy', 'strings'],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'prb_ac_abc300_a',
    platform: 'ATCODER',
    externalId: 'abc300_a',
    title: 'A - N-choice question',
    url: 'https://atcoder.jp/contests/abc300/tasks/abc300_a',
    difficulty: 100,
    tags: ['implementation'],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'prb_cf_1900B',
    platform: 'CODEFORCES',
    externalId: '1900B',
    title: 'Laura and Operations',
    url: 'https://codeforces.com/problemset/problem/1900/B',
    difficulty: 1100,
    tags: ['math', 'constructive algorithms'],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 'prb_in_use',
    platform: 'CODEFORCES',
    externalId: '2000C',
    title: 'Problem In Active Contest',
    url: 'https://codeforces.com/problemset/problem/2000/C',
    difficulty: 1500,
    tags: ['data structures'],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

export const mockStatement: ProblemStatement = {
  html: `
    <div class="problem-statement">
      <p>Given an integer $N$ and two numbers $A$ and $B$, compute the sum $A + B$.</p>
      <p>Mathematical formula: $$\\sum_{i=1}^{N} i = \\frac{N(N+1)}{2}$$</p>
    </div>
  `,
  timeLimit: '2.0s',
  memoryLimit: '256MB',
  sampleCases: [
    {
      input: '3 1 2\n2 3 5',
      output: '2',
      explanation: 'The second option gives 1 + 2 = 3.',
    },
    {
      input: '4 5 5\n1 2 3 10',
      output: '4',
      explanation: 'The fourth option is 10.',
    },
  ],
};

export const mockProblemSets: ProblemSet[] = [
  {
    id: 'set_standard_dp',
    ownerId: 'usr_adm_999',
    name: 'Dynamic Programming Foundations',
    description: 'Classic dynamic programming problems from Codeforces and AtCoder.',
    visibility: 'PUBLIC',
    problemCount: 2,
    createdAt: '2026-01-10T00:00:00Z',
    updatedAt: '2026-01-10T00:00:00Z',
    items: [
      {
        problemSetId: 'set_standard_dp',
        problemId: 'prb_cf_1000A',
        position: 1,
        problem: mockProblems[0],
      },
      {
        problemSetId: 'set_standard_dp',
        problemId: 'prb_ac_abc300_a',
        position: 2,
        problem: mockProblems[1],
      },
    ],
  },
];

export const mockContests: Contest[] = [
  {
    id: 'con_active_icpc',
    ownerId: 'usr_adm_999',
    ownerUsername: 'root_admin',
    name: 'Weekly Practice Contest #42',
    description: 'A 2-hour ICPC formatted virtual contest with live scoreboard.',
    startAt: new Date(Date.now() - 30 * 60 * 1000).toISOString(), // started 30 mins ago
    endAt: new Date(Date.now() + 90 * 60 * 1000).toISOString(), // ends in 90 mins
    visibility: 'PUBLIC',
    scoringType: 'ICPC',
    publicationStatus: 'PUBLISHED',
    state: 'ACTIVE',
    participantCount: 12,
    isParticipant: true,
    createdAt: '2026-01-15T00:00:00Z',
    updatedAt: '2026-01-15T00:00:00Z',
    problems: [
      {
        contestId: 'con_active_icpc',
        problemId: 'prb_cf_1000A',
        position: 1,
        label: 'A',
        points: 100,
        problem: mockProblems[0],
      },
      {
        contestId: 'con_active_icpc',
        problemId: 'prb_ac_abc300_a',
        position: 2,
        label: 'B',
        points: 200,
        problem: mockProblems[1],
      },
    ],
  },
  {
    id: 'con_upcoming_01',
    ownerId: 'usr_adm_999',
    ownerUsername: 'root_admin',
    name: 'Grand Prix of Tokyo',
    description: 'Upcoming high-stakes team competition.',
    startAt: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(), // starts tomorrow
    endAt: new Date(Date.now() + 29 * 60 * 60 * 1000).toISOString(),
    visibility: 'PUBLIC',
    scoringType: 'ICPC',
    publicationStatus: 'PUBLISHED',
    state: 'UPCOMING',
    participantCount: 45,
    isParticipant: false,
    createdAt: '2026-01-16T00:00:00Z',
    updatedAt: '2026-01-16T00:00:00Z',
    problems: [
      {
        contestId: 'con_upcoming_01',
        problemId: 'prb_cf_1900B',
        position: 1,
        label: 'A',
        points: 100,
        problem: mockProblems[2],
      },
    ],
  },
  {
    id: 'con_finished_99',
    ownerId: 'usr_adm_999',
    ownerUsername: 'root_admin',
    name: 'Winter Warmup 2026',
    description: 'Past finished contest archive.',
    startAt: '2026-01-01T12:00:00Z',
    endAt: '2026-01-01T17:00:00Z',
    visibility: 'PUBLIC',
    scoringType: 'ICPC',
    publicationStatus: 'PUBLISHED',
    state: 'FINISHED',
    participantCount: 88,
    isParticipant: true,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    problems: [
      {
        contestId: 'con_finished_99',
        problemId: 'prb_cf_1000A',
        position: 1,
        label: 'A',
        points: 100,
        problem: mockProblems[0],
      },
    ],
  },
  {
    id: 'con_draft_01',
    ownerId: 'usr_adm_999',
    ownerUsername: 'root_admin',
    name: 'Draft Secret Contest',
    description: 'Draft contest that should only be visible to administrators.',
    startAt: new Date(Date.now() + 48 * 60 * 60 * 1000).toISOString(),
    endAt: new Date(Date.now() + 50 * 60 * 60 * 1000).toISOString(),
    visibility: 'PUBLIC',
    scoringType: 'ICPC',
    publicationStatus: 'DRAFT',
    state: 'UPCOMING',
    participantCount: 0,
    isParticipant: false,
    createdAt: '2026-01-20T00:00:00Z',
    updatedAt: '2026-01-20T00:00:00Z',
    problems: [
      {
        contestId: 'con_draft_01',
        problemId: 'prb_cf_1000A',
        position: 1,
        label: 'A',
        points: 100,
        problem: mockProblems[0],
      },
    ],
  },
];

export const mockStandings: Standings = {
  contestId: 'con_active_icpc',
  contestName: 'Weekly Practice Contest #42',
  scoringType: 'ICPC',
  problems: [
    {
      problemId: 'prb_cf_1000A',
      label: 'A',
      title: 'Codehorses T-shirts',
      position: 1,
    },
    {
      problemId: 'prb_ac_abc300_a',
      label: 'B',
      title: 'A - N-choice question',
      position: 2,
    },
  ],
  standings: [
    {
      rank: 1,
      userId: 'usr_reg_123',
      username: 'tourist_fan',
      solvedCount: 2,
      totalPenalty: 45,
      problemScores: {
        prb_cf_1000A: {
          solved: true,
          attempts: 1,
          firstSolvedAtMinutes: 15,
          penalty: 15,
        },
        prb_ac_abc300_a: {
          solved: true,
          attempts: 2,
          firstSolvedAtMinutes: 30,
          penalty: 50,
        },
      },
    },
    {
      rank: 2,
      userId: 'usr_rival_456',
      username: 'speed_coder',
      solvedCount: 1,
      totalPenalty: 22,
      problemScores: {
        prb_cf_1000A: {
          solved: true,
          attempts: 1,
          firstSolvedAtMinutes: 22,
          penalty: 22,
        },
        prb_ac_abc300_a: {
          solved: false,
          attempts: 3,
        },
      },
    },
  ],
};

export const mockSubmissions: Submission[] = [
  {
    id: 'sub_001',
    userId: 'usr_reg_123',
    username: 'tourist_fan',
    problemId: 'prb_cf_1000A',
    contestId: 'con_active_icpc',
    platform: 'CODEFORCES',
    externalSubmissionId: 'cf_28001',
    language: 'cpp23',
    sourceCode: `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios::sync_with_stdio(false);\n    cin.tie(nullptr);\n    cout << "Accepted Solution" << endl;\n    return 0;\n}`,
    status: 'ACCEPTED',
    sourceUrl: 'https://codeforces.com/contest/1000/submission/28001',
    submittedAt: new Date(Date.now() - 15 * 60 * 1000).toISOString(),
    judgedAt: new Date(Date.now() - 14 * 60 * 1000).toISOString(),
    problemTitle: 'Codehorses T-shirts',
    metadata: {
      executionTimeMs: 45,
      memoryBytes: 1048576,
      passedCount: 25,
      totalCount: 25,
    },
  },
  {
    id: 'sub_002',
    userId: 'usr_reg_123',
    username: 'tourist_fan',
    problemId: 'prb_ac_abc300_a',
    contestId: 'con_active_icpc',
    platform: 'ATCODER',
    externalSubmissionId: 'ac_99124',
    language: 'python3',
    sourceCode: `import sys\n\ndef main():\n    lines = sys.stdin.read().split()\n    # Incorrect logic for test\n    print(-1)\n\nif __name__ == '__main__':\n    main()`,
    status: 'WRONG_ANSWER',
    sourceUrl: 'https://atcoder.jp/contests/abc300/submissions/99124',
    submittedAt: new Date(Date.now() - 25 * 60 * 1000).toISOString(),
    judgedAt: new Date(Date.now() - 24 * 60 * 1000).toISOString(),
    problemTitle: 'A - N-choice question',
    metadata: {
      executionTimeMs: 120,
      memoryBytes: 15728640,
      passedCount: 3,
      totalCount: 15,
      error: 'Wrong Answer on test 4: expected 2, found -1',
    },
  },
  {
    id: 'sub_003',
    userId: 'usr_reg_123',
    username: 'tourist_fan',
    problemId: 'prb_cf_1900B',
    contestId: null,
    platform: 'CODEFORCES',
    externalSubmissionId: 'cf_28099',
    language: 'java21',
    sourceCode: `import java.util.Scanner;\n\npublic class Solution {\n    public static void main(String[] args) {\n        Scanner sc = new Scanner(System.in);\n        System.out.println("Processing...");\n    }\n}`,
    status: 'JUDGING',
    sourceUrl: 'https://codeforces.com/contest/1900/submission/28099',
    submittedAt: new Date().toISOString(),
    judgedAt: null,
    problemTitle: 'Laura and Operations',
    metadata: {},
  },
];

export const mockAdminStats = {
  totalProblems: 42,
  totalProblemSets: 8,
  totalContests: 15,
  activeContests: 2,
  upcomingContests: 5,
  totalUsers: 120,
};

export const mockUsersList: User[] = [
  mockAdminUser,
  mockRegularUser,
  {
    id: 'usr_inactive_789',
    email: 'banned@example.com',
    username: 'bad_actor',
    role: 'USER',
    isActive: false,
    createdAt: '2026-01-05T00:00:00Z',
    updatedAt: '2026-01-05T00:00:00Z',
  },
];
