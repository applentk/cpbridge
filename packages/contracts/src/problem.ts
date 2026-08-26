export type PlatformType = 'CODEFORCES' | 'ATCODER';

export type LanguageId = 'cpp23' | 'python3' | 'java21';

export const LANGUAGE_LABELS: Record<LanguageId, string> = {
  cpp23: 'C++23 (GCC)',
  python3: 'Python 3',
  java21: 'Java 21'
};

export function formatLanguageName(lang: string): string {
  if (!lang) return '';
  return LANGUAGE_LABELS[lang as LanguageId] || lang;
}

export interface SampleCase {
  input: string;
  output: string;
  explanation?: string;
}

export interface ProblemStatement {
  html: string;
  timeLimit?: string;
  memoryLimit?: string;
  sampleCases: SampleCase[];
  note?: string;
}

export interface Problem {
  id: string;
  platform: PlatformType;
  externalId: string;
  title: string;
  url: string;
  difficulty: number | null;
  tags: string[];
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface ImportProblemRequest {
  url: string;
}

export interface CreateCustomProblemRequest {
  platform: PlatformType;
  externalId: string;
  title: string;
  url: string;
  difficulty?: number | null;
  tags?: string[];
  statement?: string;
  timeLimit?: string;
  memoryLimit?: string;
  sampleCases?: SampleCase[];
}

export interface UpdateProblemRequest {
  title?: string;
  url?: string;
  difficulty?: number | null;
  tags?: string[];
  metadata?: Record<string, unknown>;
}

export interface ExtractStatementRequest {
  rawContent: string;
  platform?: PlatformType;
}

export interface ExtractStatementResponse {
  title?: string;
  statement: string;
  timeLimit?: string;
  memoryLimit?: string;
  sampleCases: SampleCase[];
}

export interface ProblemFilter {
  platform?: PlatformType;
  query?: string;
  minDifficulty?: number;
  maxDifficulty?: number;
  tag?: string;
  limit?: number;
  offset?: number;
}
