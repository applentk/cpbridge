export interface ProblemScore {
  problemId: string;
  label: string;
  solved: boolean;
  attempts: number;
  penaltyMinutes: number;
  firstSolvedAtMinutes?: number | null;
}

export interface ParticipantScore {
  userId: string;
  username: string;
  rank: number;
  solvedCount: number;
  totalPenalty: number;
  problemScores: Record<string, ProblemScore>;
}

export interface Standings {
  contestId: string;
  scoringType: 'SIMPLE' | 'ICPC';
  standings: ParticipantScore[];
  upsolveStandings?: ParticipantScore[];
  problems: {
    problemId: string;
    label: string;
    title: string;
    platform: string;
  }[];
  generatedAt: string;
}
