import { describe, expect, it } from "vitest";
import { ChatRound, chatHistory } from "./chat-history";

function answered(id: number): ChatRound {
  return { id, question: `问题 ${id}`, status: "answered", result: { answer: `回答 ${id}`, citations: [], index_state: "ready" } };
}

describe("chat context", () => {
  it("keeps only the latest three completed rounds and excludes unfinished messages", () => {
    const history = chatHistory([
      answered(1), answered(2),
      { id: 3, question: "失败的问题", status: "failed", error: "503" },
      answered(4),
      { id: 5, question: "停止的问题", status: "stopped" },
      answered(6),
      { id: 7, question: "正在回答", status: "pending" }
    ]);
    expect(history).toEqual([
      { role: "user", content: "问题 2" }, { role: "assistant", content: "回答 2" },
      { role: "user", content: "问题 4" }, { role: "assistant", content: "回答 4" },
      { role: "user", content: "问题 6" }, { role: "assistant", content: "回答 6" }
    ]);
  });

  it("bounds history by codepoints without splitting supplementary characters or changing the displayed answer", () => {
    const round = answered(1);
    round.result!.answer = "😀".repeat(1100);
    const history = chatHistory([round]);
    expect(history[1].content).toBe("😀".repeat(1000));
    expect(round.result!.answer).toBe("😀".repeat(1100));
    expect(Object.keys(history[1])).toEqual(["role", "content"]);
    expect(chatHistory([])).toEqual([]);
  });
});
