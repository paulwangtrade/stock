/** 研究页候选池 HTTP API */

export type CandidatePoolItem = {
  stock_code: string
  stock_name: string
  signal_score: number
  direction: string
  reason?: string
  signal_tag?: string
  price?: string
}

export type CandidatePoolListResponse = {
  code: number
  data: CandidatePoolItem[]
  total: number
  snapshot_time: string
  snapshot_id?: number
  threshold: number
  message?: string
}

export async function getCandidatePool(minScore?: number): Promise<CandidatePoolListResponse> {
  const q = minScore != null && minScore > 0 ? `?min_score=${encodeURIComponent(String(minScore))}` : ''
  const res = await fetch(`/api/candidate_pool/list${q}`)
  if (!res.ok) {
    throw new Error(`候选池请求失败: HTTP ${res.status}`)
  }
  const body = await res.json()
  // 兼容旧版直接返回数组
  if (Array.isArray(body)) {
    return {
      code: 0,
      data: body,
      total: body.length,
      snapshot_time: '',
      threshold: minScore || 60,
    }
  }
  return {
    code: body?.code ?? 0,
    data: Array.isArray(body?.data) ? body.data : [],
    total: Number(body?.total) || 0,
    snapshot_time: body?.snapshot_time || '',
    snapshot_id: Number(body?.snapshot_id) || 0,
    threshold: Number(body?.threshold) || 60,
    message: body?.message || '',
  }
}
