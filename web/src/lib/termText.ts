// Turning the terminal's scrollback into plain text to copy or save (#142).

export interface BufferLine {
  /** The line's text as the screen shows it, trailing blanks trimmed. */
  text: string
  /** True when this line continues the one before it (a long line the terminal wrapped). */
  wrapped: boolean
}

export const CUT_NOTICE = '[Earlier output is not included: only the most recent scrollback is kept.]'

// Controls other than tab and newline have no place in copied text.
// eslint-disable-next-line no-control-regex
const CONTROLS = /[\u0000-\u0008\u000b-\u001f\u007f-\u009f]/g

/** Removes terminal control characters; escape sequences never reach the buffer, but stray bytes can. */
export function stripControls(text: string): string {
  return text.replace(CONTROLS, '')
}

/**
 * Joins wrapped lines back into the lines the program wrote, drops the blank
 * lines after the last output, and says so at the top when older output was
 * dropped from the scrollback.
 */
export function bufferText(lines: BufferLine[], cut: boolean): string {
  const out: string[] = []
  for (const line of lines) {
    const text = stripControls(line.text)
    if (line.wrapped && out.length > 0) out[out.length - 1] += text
    else out.push(text)
  }
  while (out.length > 0 && out[out.length - 1].trim() === '') out.pop()
  const body = out.join('\n')
  return cut ? `${CUT_NOTICE}\n${body}` : body
}

/** True when the buffer is full, which is when xterm has started dropping its oldest lines. */
export function wasCut(length: number, rows: number, scrollback: number): boolean {
  return length >= rows + scrollback
}

/** A file name for saved output that names the session and nothing else. */
export function outputFileName(sessionId: string): string {
  return `session-${sessionId.replace(/[^A-Za-z0-9_-]/g, '')}-output.txt`
}
