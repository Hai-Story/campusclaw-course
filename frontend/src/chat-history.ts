import type { AskResult, HistoryTurn } from "./api";

export interface ChatRound {
  id: number;
  question: string;
  status: "pending" | "answered" | "failed" | "stopped";
  result?: AskResult;
  error?: string;
}

// Failed and cancelled questions never become model context. Citation metadata
// belongs to its round and is deliberately excluded from the history payload.
export function chatHistory(rounds: ChatRound[]): HistoryTurn[] {
  return rounds
    .filter((round) => round.status === "answered" && round.result)
    .slice(-3)
    .flatMap((round): HistoryTurn[] => [
      { role: "user", content: round.question },
      { role: "assistant", content: Array.from(round.result!.answer).slice(0, 1000).join("") }
    ]);
}
