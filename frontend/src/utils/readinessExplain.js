/** Readiness / QualityGate 英文 finding → 中文解释（仅展示，不改判定逻辑） */

const BY_CODE = {
  LIMIT_PRICE_MISSING: {
    title: '缺少买入限价',
    impact: '无法生成完整订单规格',
    reason: '非skip买入项缺少limit_price',
  },
  TARGET_VOLUME_BELOW_LOT: {
    title: '目标股数不足一手',
    impact: '无法生成完整订单规格',
    reason: '非skip买入项目标股数 < 100',
  },
  PRICED_VOLUME_NOT_MATERIALIZED: {
    title: '定价后股数未物化',
    impact: '无法生成完整订单规格',
    reason: '已标为 priced，但目标股数尚未物化到有效手数',
  },
  NO_TRADEABLE_ITEMS: {
    title: '无可交易明细',
    impact: '当前计划不可形成有效订单',
    reason: '没有 priced + 限价>0 + 股数≥一手 的可交易项',
  },
  LEGACY_NO_INTENT: {
    title: '缺少 Execution Intent 合约',
    impact: '无法按 Intent 规格执行',
    reason: '旧版计划 pricing_policy_version < 1',
  },
  INTENT_NOT_MATERIALIZED: {
    title: 'Intent 尚未物化',
    impact: '当前计划不可Approve / Freeze（执行准备不足）',
    reason: '买入项仍为 selected，尚未完成早盘物化',
  },
  PLAN_NIL: {
    title: '交易计划为空',
    impact: '无法评估就绪度',
    reason: '计划元数据缺失',
  },
  ENTRY_PRICE_MISSING: {
    title: '缺少入场价/锚定价',
    impact: '质量门禁阻断执行准备',
    reason: '买入项缺少 limit_price / 锚定价',
  },
  POSITION_CONFLICT: {
    title: '持仓冲突',
    impact: '质量门禁阻断',
    reason: '与现有持仓规则冲突',
  },
  SECTOR_CONCENTRATION: {
    title: '板块集中度过高',
    impact: '质量门禁阻断',
    reason: '行业/板块集中度超过阈值',
  },
  OPEN_GAP_PENDING: {
    title: '开盘缺口待评估',
    impact: '质量门禁尚未通过',
    reason: '开盘缺口评估待完成或未通过',
  },
  PLAN_COMPLETENESS: {
    title: '计划完整性不足',
    impact: '影响观察完整性（通常为 Warning）',
    reason: '字段或明细不完整（如名称/行业/锚定价）',
  },
}

const BY_RULE = {
  'IR-LIMIT': {
    title: '限价规则未满足',
    impact: '无法生成完整订单规格',
    reason: '限价相关就绪度检查未通过',
  },
  'IR-VOLUME': {
    title: '股数/手数规则未满足',
    impact: '无法生成完整订单规格',
    reason: '股数相关就绪度检查未通过',
  },
  'IR-PRICED-VOLUME': {
    title: '定价股数规则未满足',
    impact: '无法生成完整订单规格',
    reason: '定价后股数物化相关规则未满足',
  },
  'IR-EMPTY': {
    title: '无可交易明细',
    impact: '当前计划不可形成有效订单',
    reason: '无可交易明细',
  },
  'IR-LEGACY': {
    title: '旧版 Intent 合约缺失',
    impact: '无法按 Intent 规格执行',
    reason: '旧版计划 Intent 合约缺失',
  },
  'IR-STAGE': {
    title: '生命周期阶段未完成物化',
    impact: '当前计划不可Approve / Freeze（执行准备不足）',
    reason: '生命周期阶段尚未完成物化',
  },
  'IR-META': {
    title: '计划元数据异常',
    impact: '无法完整评估就绪度',
    reason: '计划元数据异常',
  },
}

/**
 * @param {{ code?: string, rule_code?: string, message?: string }} finding
 * @returns {{ title: string, impact: string, reason: string }}
 */
export function describeReadinessFinding(finding) {
  const code = String(finding?.code || '').trim()
  if (code && BY_CODE[code]) return BY_CODE[code]
  const rule = String(finding?.rule_code || '').trim()
  if (rule && BY_RULE[rule]) return BY_RULE[rule]
  const msg = String(finding?.message || '').trim()
  return {
    title: '就绪度检查未通过',
    impact: '影响执行准备条件',
    reason: msg || '保留英文原文以便排查',
  }
}

/**
 * @param {{ code?: string, rule_code?: string }} finding
 * @returns {string}
 */
export function explainReadinessFinding(finding) {
  return describeReadinessFinding(finding).reason
}
