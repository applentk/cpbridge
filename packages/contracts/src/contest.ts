import type { Problem } from './problem.js';

export type ContestState = 'UPCOMING' | 'ACTIVE' | 'FINISHED';
export type ScoringType = 'SIMPLE' | 'ICPC';

export interface ContestProblem {
  contestId: string;
  problemId: string;
  position: number;
  label: string;
  points?: number | null;
  problem?: Problem;
}

export interface ContestParticipant {
  contestId: string;
  userId: string;
  joinedAt: string;
  username?: string;
}

export interface Contest {
  id: string;
  ownerId: string;
  ownerUsername?: string;
  name: string;
  description: string;
  startAt: string;
  endAt: string;
  visibility: 'PUBLIC' | 'UNLISTED' | 'PRIVATE';
  scoringType: ScoringType;
  createdAt: string;
  updatedAt: string;
  state: ContestState;
  problems?: ContestProblem[];
  participantCount?: number;
  isParticipant?: boolean;
}

export interface CreateContestRequest {
  problemSetId: string;
  name: string;
  description?: string;
  startAt: string;
  endAt: string;
  visibility?: 'PUBLIC' | 'UNLISTED' | 'PRIVATE';
  scoringType?: ScoringType;
}
