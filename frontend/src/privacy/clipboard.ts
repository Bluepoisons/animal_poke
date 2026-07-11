/**
 * 剪贴板 / 下载：失败绝不伪装成功（AP-068）。
 */

export type CopyResult =
  | { ok: true; method: 'clipboard' | 'download' }
  | { ok: false; error: string }

/** 将文本写入剪贴板；不可用或失败返回 ok:false */
export async function copyTextToClipboard(
  text: string,
  clipboard: { writeText?: (v: string) => Promise<void> } | null | undefined = typeof navigator !==
  'undefined'
    ? navigator.clipboard
    : null,
): Promise<CopyResult> {
  if (!text) return { ok: false, error: 'empty_payload' }
  if (!clipboard || typeof clipboard.writeText !== 'function') {
    return { ok: false, error: 'clipboard_unavailable' }
  }
  try {
    await clipboard.writeText(text)
    return { ok: true, method: 'clipboard' }
  } catch (e) {
    const message = e instanceof Error ? e.message : 'clipboard_write_failed'
    return { ok: false, error: message }
  }
}

/** 触发浏览器下载 JSON 文件（clipboard 失败时的恢复路径） */
export function downloadTextFile(
  text: string,
  filename: string,
  doc: Document | null = typeof document !== 'undefined' ? document : null,
): CopyResult {
  if (!text) return { ok: false, error: 'empty_payload' }
  if (!doc || typeof URL === 'undefined' || typeof Blob === 'undefined') {
    return { ok: false, error: 'download_unavailable' }
  }
  try {
    const blob = new Blob([text], { type: 'application/json;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = doc.createElement('a')
    a.href = url
    a.download = filename
    a.rel = 'noopener'
    a.style.display = 'none'
    doc.body.appendChild(a)
    a.click()
    a.remove()
    // 延迟 revoke，避免部分浏览器下载中断
    setTimeout(() => {
      try {
        URL.revokeObjectURL(url)
      } catch {
        /* ignore */
      }
    }, 2_000)
    return { ok: true, method: 'download' }
  } catch (e) {
    const message = e instanceof Error ? e.message : 'download_failed'
    return { ok: false, error: message }
  }
}

/**
 * 优先剪贴板，失败则尝试下载；两者都失败才 ok:false。
 * 调用方仅在 ok:true 时展示成功 toast。
 */
export async function exportTextPayload(
  text: string,
  filename: string,
): Promise<CopyResult> {
  const clip = await copyTextToClipboard(text)
  if (clip.ok) return clip
  const dl = downloadTextFile(text, filename)
  if (dl.ok) return dl
  return { ok: false, error: clip.error || dl.error || 'export_failed' }
}
