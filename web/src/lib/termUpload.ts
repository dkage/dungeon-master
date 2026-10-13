// Files pasted or dropped onto the terminal become a path in the prompt: the
// browser never reveals where a dropped file lives, so the server keeps a
// private copy for the session and the terminal inserts that path (#86, #90).

/** Mirrors the server's limit, so a big file is refused before it is read. */
export const MAX_UPLOAD_BYTES = 10 * 1024 * 1024

const SAFE_PATH = /^[A-Za-z0-9_./:\\@%+=,~-]+$/

/**
 * Quotes a path for a prompt or a shell: bare when it has only safe
 * characters, double-quoted for a Windows path, otherwise single-quoted with
 * any quote inside it closed and reopened. The Claude prompt reads any of
 * these as the file (probe on #89).
 */
export function quotePath(path: string): string {
  if (SAFE_PATH.test(path)) return path
  if (/^[A-Za-z]:\\/.test(path) && !path.includes('"')) return `"${path}"`
  return `'${path.replace(/'/g, `'\\''`)}'`
}

/** Joins several paths into the one string inserted at the cursor. */
export function pathsText(paths: string[]): string {
  return paths.map(quotePath).join(' ')
}

/** Standard base64 of a file's bytes, in chunks so a large file does not overflow the call stack. */
export async function fileToBase64(file: Blob): Promise<string> {
  const bytes = new Uint8Array(await file.arrayBuffer())
  let bin = ''
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  return btoa(bin)
}

/** Why a file will not be sent, or null when it may be. */
export function refusal(file: { name: string; size: number }): string | null {
  if (file.size === 0) return `${file.name || 'The file'} is empty`
  if (file.size > MAX_UPLOAD_BYTES) return `${file.name || 'The file'} is larger than 10 MB`
  return null
}

/** The files a paste or drop carries, in order; empty when it is plain text. */
export function filesOf(data: DataTransfer | null): File[] {
  return data ? Array.from(data.files ?? []) : []
}
