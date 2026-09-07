/** K 线买卖点信号参数：默认值、合并、转 computeFullSignals 选项 */

import { DEFAULT_QUANT_AUTOMATION, mergeQuantAutomation } from './quantAutomationSettings'

export const DEFAULT_SCREEN_STRATEGY_ID = 'default'
export const DEFAULT_SCREEN_STRATEGY_NAME = '默认参数预设'

export const DEFAULT_SIGNAL_SETTINGS = {
  automation: DEFAULT_QUANT_AUTOMATION,
  display: {
    watchlistCardSignalTint: false,
    watchlistActionPopup: false,
    followDateGroupEnabled: true,
    dateGroupRetainDays: 30,
    recentBuyDays: 15,
    recentSellDays: 5,
    /** 自选股票数量上限 */
    maxFollowCount: 100,
  },
  common: {
    rsiPeriod: 14,
    iceThreshold: 30,
    overbought: 70,
    lookback: 5,
    maPeriod: 20,
    volPeriod: 5,
    requireIndexBull: true,
  },
  sell: {
    /** 连续 N 日收在 MA20 下才标减（1=首次跌破当日） */
    reduceConfirmDays: 2,
    /** 提前减：MA20 确认前的走弱信号 */
    reduceEarlyEnabled: true,
    /** 连续 N 日收在 MA5 下且仍在 MA20 附近 */
    reduceEarlyMa5Days: 2,
    /** 单日跌幅阈值（前一日在 MA20 上） */
    reduceEarlyMinDropPct: 0.045,
    /** 跌破近 N 日低点（前一日在 MA20 上） */
    reduceEarlyBreakLowDays: 5,
    /** 近 N 日内须在 MA20 上（确认曾处于升势） */
    reduceEarlyUptrendLookback: 8,
    reduceEarlyRequireUptrend: true,
    /** RSI 在止阈值上方至少 N 日才允许标止 */
    takeProfitMinOverboughtDays: 1,
    /** 止：相对最近买点至少该浮盈比例才标 */
    takeProfitMinGainPct: 0.05,
    /** 止：自买点以来最高价回撤至少该比例（过滤整理中的小止） */
    takeProfitMinPullbackFromPeakPct: 0.04,
    /** 止：要求自阶段高点已有回撤 */
    takeProfitRequirePullbackFromPeak: true,
    /** 止：浮盈门槛总开关 */
    sellProfitGateEnabled: true,
    /** 保护期内收盘跌破买点当日低点仍可标减 */
    breakEntryLowEnablesReduce: true,
    /** 跌破买点低点判定的回溯窗口 */
    breakEntryLowLookback: 15,
    /** 各买点标记后止/减保护（0=不保护） */
    protectAfterBuy: 0,
    protectAfterStrong: 0,
    protectAfterTrend: 0,
    protectAfterBreakout: 0,
    protectAfterRebound: 0,
    protectAfterReversal: 0,
    /** 同股两次「减」最小间隔 */
    reduceMinGap: 8,
    /** 同股两次「止」最小间隔 */
    takeProfitMinGap: 8,
    /** 兼容旧配置：未分项时各类型 fallback */
    protectAfterEntryDays: 0,
  },
  buy: {
    minBodyPct: 0.01,
  },
  strong: {
    volMult: 1.25,
    confirmDays: 2,
    iceMinGap: 4,
    requireStockAboveMa20: true,
    requireMa20Rising: true,
  },
  trend: {
    rsiMin: 45,
    rsiMax: 70,
    touchMaPct: 0.02,
    /** 收盘相对 MA5 涨幅上限；0=不限制 */
    maxCloseAboveMa5Pct: 0,
    minUpBars: 2,
    minGap: 5,
    /** 震荡/粘合/近端有止减时不标趋 */
    chopFilterEnabled: true,
    /** MA5 须高于 MA10 至少该比例，否则视为均线粘合 */
    minMaSpreadPct: 0.01,
    /** 近 N 日内有止/减则不标趋 */
    chopSellLookback: 8,
    /** 近 N 日振幅低于 chopMaxRangePct 视为横盘 */
    chopRangeDays: 5,
    chopMaxRangePct: 0.08,
    /** 开启：回踩日满足条件后，须次日收阳才在次日 K 线标「趋」 */
    requireNextDayYang: true,
  },
  reversal: {
    enabled: true,
    crossMaPeriod: 60,
    minBodyPct: 0.01,
    rsiMin: 45,
    rsiMax: 70,
    requireMa20FlatOrUp: true,
    ma20LookbackDays: 5,
    requireMa5AboveMa10: true,
    requireCloseAboveMa5: true,
    recentBelowDays: 5,
    recentBelowMinRatio: 0.6,
    minGap: 5,
  },
  breakout: {
    boxPeriod: 20,
    maxRangePct: 0.2,
    volMult: 1.25,
    minGap: 15,
    breakBuffer: 0.005,
    confirmDays: 2,
    maxUpperWickRatio: 0.55,
  },
  rebound: {
    sellLookback: 6,
    minDaysAfterSell: 1,
    minBodyPct: 0.015,
    maxRsi: 60,
    minReboundPct: 0.05,
    buyAdjacentDays: 2,
    minGap: 6,
    confirmDays: 2,
    maxUpperWickRatio: 0.5,
    screenMaxRsi: 60,
  },
}

function cloneDefaultSignalSettingsCore() {
  const base = deepClone(DEFAULT_SIGNAL_SETTINGS)
  delete base.automation
  delete base.display
  delete base.activeScreenStrategyId
  delete base.screenStrategies
  return base
}

/** 设置页字段定义：scale=100 表示 UI 以 % 展示（存小数） */
export const SIGNAL_PARAM_SECTIONS = [
  {
    key: 'common',
    title: '通用',
    groups: [
      { id: 'core', title: 'RSI / 均线', desc: '冰、买、止、减共用的基础指标' },
      { id: 'env', title: '强买环境' },
    ],
    fields: [
      { key: 'rsiPeriod', label: 'RSI 周期', group: 'core', type: 'int', min: 5, max: 30, step: 1 },
      { key: 'iceThreshold', label: '冰点阈值', group: 'core', type: 'number', min: 10, max: 45, step: 1 },
      { key: 'overbought', label: '止 · RSI 阈值', group: 'core', type: 'number', min: 60, max: 90, step: 1 },
      { key: 'lookback', label: '出冰回溯', group: 'core', type: 'int', min: 1, max: 20, step: 1, suffix: '日' },
      { key: 'maPeriod', label: '减 · MA 周期', group: 'core', type: 'int', min: 5, max: 60, step: 1 },
      { key: 'volPeriod', label: '均量周期', group: 'core', type: 'int', min: 3, max: 20, step: 1 },
      { key: 'requireIndexBull', label: '强买须上证 MA20 上', group: 'env', type: 'bool' },
    ],
  },
  {
    key: 'sell',
    title: '止 / 减',
    groups: [
      { id: 'reduce', title: '减 · 主规则', desc: '结构走坏时标减（主卖信号）' },
      { id: 'reduceEarly', title: '减 · 提前预警', desc: 'MA20 未破前的走弱；可整体关闭' },
      { id: 'takeProfit', title: '止 · 止盈', desc: '有浮盈时的辅助止盈，不当清仓依据' },
      { id: 'protect', title: '买点后保护', desc: '各买点后 N 日内默认不标止/减；0 = 关闭' },
    ],
    fields: [
      {
        key: 'reduceConfirmDays',
        label: '连续破 MA20',
        group: 'reduce',
        type: 'int',
        min: 1,
        max: 5,
        step: 1,
        suffix: '日',
        hint: '1 = 首次跌破当日',
      },
      { key: 'breakEntryLowEnablesReduce', label: '破买点低点仍标', group: 'reduce', type: 'bool' },
      { key: 'breakEntryLowLookback', label: '破低点回溯', group: 'reduce', type: 'int', min: 5, max: 30, step: 1, suffix: '日' },
      { key: 'reduceMinGap', label: '同股间隔', group: 'reduce', type: 'int', min: 0, max: 20, step: 1, suffix: '日' },
      { key: 'reduceEarlyEnabled', label: '启用提前预警', group: 'reduceEarly', type: 'bool' },
      {
        key: 'reduceEarlyMa5Days',
        label: '连续破 MA5',
        group: 'reduceEarly',
        type: 'int',
        min: 1,
        max: 4,
        step: 1,
        suffix: '日',
      },
      {
        key: 'reduceEarlyMinDropPct',
        label: '大阴跌幅',
        group: 'reduceEarly',
        type: 'number',
        min: 0.02,
        max: 0.1,
        step: 0.005,
        scale: 100,
        suffix: '%',
        uiPrecision: 1,
      },
      {
        key: 'reduceEarlyBreakLowDays',
        label: '破近端低点',
        group: 'reduceEarly',
        type: 'int',
        min: 2,
        max: 10,
        step: 1,
        suffix: '日',
      },
      {
        key: 'takeProfitMinOverboughtDays',
        label: 'RSI 超买停留',
        group: 'takeProfit',
        type: 'int',
        min: 1,
        max: 5,
        step: 1,
        suffix: '日',
      },
      { key: 'takeProfitMinGap', label: '同股间隔', group: 'takeProfit', type: 'int', min: 0, max: 20, step: 1, suffix: '日' },
      { key: 'sellProfitGateEnabled', label: '须满足浮盈门槛', group: 'takeProfit', type: 'bool' },
      { key: 'takeProfitRequirePullbackFromPeak', label: '须自高点回撤', group: 'takeProfit', type: 'bool' },
      {
        key: 'takeProfitMinGainPct',
        label: '最低浮盈',
        group: 'takeProfit',
        type: 'number',
        min: 0.02,
        max: 0.2,
        step: 0.01,
        scale: 100,
        suffix: '%',
        uiPrecision: 0,
      },
      {
        key: 'takeProfitMinPullbackFromPeakPct',
        label: '自高点回撤',
        group: 'takeProfit',
        type: 'number',
        min: 0.02,
        max: 0.15,
        step: 0.005,
        scale: 100,
        suffix: '%',
        uiPrecision: 1,
      },
      { key: 'protectAfterTrend', label: '趋', group: 'protect', type: 'int', min: 0, max: 10, step: 1, suffix: '日' },
      { key: 'protectAfterStrong', label: '强', group: 'protect', type: 'int', min: 0, max: 10, step: 1, suffix: '日' },
      { key: 'protectAfterBreakout', label: '突', group: 'protect', type: 'int', min: 0, max: 10, step: 1, suffix: '日' },
      { key: 'protectAfterReversal', label: '转', group: 'protect', type: 'int', min: 0, max: 10, step: 1, suffix: '日' },
      { key: 'protectAfterBuy', label: '买', group: 'protect', type: 'int', min: 0, max: 10, step: 1, suffix: '日' },
      { key: 'protectAfterRebound', label: '弹', group: 'protect', type: 'int', min: 0, max: 10, step: 1, suffix: '日' },
    ],
  },
  {
    key: 'strong',
    title: '强',
    fields: [
      { key: 'volMult', label: '放量倍数', type: 'number', min: 1, max: 3, step: 0.05 },
      { key: 'confirmDays', label: '确认站稳天数', type: 'int', min: 0, max: 5, step: 1, suffix: '日' },
      { key: 'iceMinGap', label: '与冰点最小间隔', type: 'int', min: 1, max: 15, step: 1, suffix: '日' },
      { key: 'requireStockAboveMa20', label: '须收在 MA20 上', type: 'bool' },
      { key: 'requireMa20Rising', label: '须 MA20 上行', type: 'bool' },
    ],
  },
  {
    key: 'trend',
    title: '趋',
    groups: [
      { id: 'entry', title: '回踩条件' },
      { id: 'chop', title: '震荡过滤', desc: '粘合 / 近端止减 / 横盘时不标趋' },
    ],
    fields: [
      { key: 'rsiMin', label: 'RSI 下限', group: 'entry', type: 'number', min: 30, max: 60, step: 1 },
      { key: 'rsiMax', label: 'RSI 上限', group: 'entry', type: 'number', min: 55, max: 80, step: 1 },
      { key: 'touchMaPct', label: '回踩 MA 容差', group: 'entry', type: 'number', min: 0.005, max: 0.03, step: 0.001, scale: 100, suffix: '%', uiPrecision: 1 },
      {
        key: 'maxCloseAboveMa5Pct',
        label: '收盘距 MA5 上限',
        group: 'entry',
        type: 'number',
        min: 0,
        max: 0.15,
        step: 0.005,
        scale: 100,
        suffix: '%',
        uiPrecision: 1,
        hint: '0 = 不限制',
      },
      { key: 'minUpBars', label: 'MA20 上方最少', group: 'entry', type: 'int', min: 1, max: 5, step: 1, suffix: '日' },
      { key: 'minGap', label: '同股间隔', group: 'entry', type: 'int', min: 1, max: 15, step: 1, suffix: '日' },
      {
        key: 'requireNextDayYang',
        label: '须次日收阳确认',
        group: 'entry',
        type: 'bool',
        hint: '开启后在次日 K 线标趋；关闭则回踩收阳当日即标',
      },
      { key: 'chopFilterEnabled', label: '启用震荡过滤', group: 'chop', type: 'bool' },
      { key: 'minMaSpreadPct', label: 'MA5-MA10 间距', group: 'chop', type: 'number', min: 0.003, max: 0.03, step: 0.001, scale: 100, suffix: '%', uiPrecision: 1 },
      { key: 'chopSellLookback', label: '止/减回溯', group: 'chop', type: 'int', min: 0, max: 20, step: 1, suffix: '日', hint: '0 = 不检查' },
      { key: 'chopRangeDays', label: '横盘观察', group: 'chop', type: 'int', min: 3, max: 10, step: 1, suffix: '日' },
      { key: 'chopMaxRangePct', label: '横盘振幅上限', group: 'chop', type: 'number', min: 0.04, max: 0.15, step: 0.005, scale: 100, suffix: '%', uiPrecision: 1 },
    ],
  },
  {
    key: 'reversal',
    title: '转',
    groups: [
      { id: 'core', title: '转势触发' },
      { id: 'filter', title: '过滤条件' },
    ],
    fields: [
      { key: 'enabled', label: '启用转势信号', group: 'core', type: 'bool' },
      { key: 'crossMaPeriod', label: '上穿均线周期', group: 'core', type: 'int', min: 20, max: 120, step: 5, suffix: '日' },
      { key: 'minBodyPct', label: '阳实体下限', group: 'core', type: 'number', min: 0.005, max: 0.05, step: 0.001, scale: 100, suffix: '%', uiPrecision: 1 },
      { key: 'minGap', label: '同股间隔', group: 'core', type: 'int', min: 3, max: 20, step: 1, suffix: '日' },
      { key: 'rsiMin', label: 'RSI 下限', group: 'filter', type: 'number', min: 30, max: 60, step: 1 },
      { key: 'rsiMax', label: 'RSI 上限', group: 'filter', type: 'number', min: 55, max: 80, step: 1 },
      { key: 'requireMa20FlatOrUp', label: 'MA20 走平/向上', group: 'filter', type: 'bool' },
      { key: 'ma20LookbackDays', label: 'MA20 对比回溯', group: 'filter', type: 'int', min: 3, max: 20, step: 1, suffix: '日' },
      { key: 'requireCloseAboveMa5', label: '收盘 > MA5', group: 'filter', type: 'bool' },
      { key: 'requireMa5AboveMa10', label: 'MA5 > MA10', group: 'filter', type: 'bool' },
      { key: 'recentBelowDays', label: '均线下观察', group: 'filter', type: 'int', min: 3, max: 15, step: 1, suffix: '日' },
      { key: 'recentBelowMinRatio', label: '均线下占比', group: 'filter', type: 'number', min: 0.4, max: 1, step: 0.1, scale: 100, suffix: '%', uiPrecision: 0 },
    ],
  },
  {
    key: 'breakout',
    title: '突',
    fields: [
      { key: 'boxPeriod', label: '箱体观察周期', type: 'int', min: 10, max: 40, step: 1, suffix: '日' },
      { key: 'maxRangePct', label: '箱体最大振幅', type: 'number', min: 0.08, max: 0.35, step: 0.01, scale: 100, suffix: '%' },
      { key: 'volMult', label: '突破放量倍数', type: 'number', min: 1, max: 3, step: 0.05 },
      { key: 'minGap', label: '同股重复间隔', type: 'int', min: 5, max: 30, step: 1, suffix: '日' },
      { key: 'breakBuffer', label: '突破缓冲', type: 'number', min: 0, max: 0.02, step: 0.001, scale: 100, suffix: '%' },
      { key: 'confirmDays', label: '突破确认天数', type: 'int', min: 0, max: 5, step: 1, suffix: '日' },
      { key: 'maxUpperWickRatio', label: '上影线占振幅上限', type: 'number', min: 0.3, max: 0.8, step: 0.05, scale: 100, suffix: '%' },
    ],
  },
  {
    key: 'rebound',
    title: '弹',
    fields: [
      { key: 'sellLookback', label: '止/减回溯窗口', type: 'int', min: 3, max: 15, step: 1, suffix: '日' },
      { key: 'minDaysAfterSell', label: '距止/减最少间隔', type: 'int', min: 0, max: 5, step: 1, suffix: '日' },
      { key: 'minBodyPct', label: '阳实体下限', type: 'number', min: 0.005, max: 0.05, step: 0.001, scale: 100, suffix: '%' },
      { key: 'maxRsi', label: 'RSI 上限', type: 'number', min: 50, max: 75, step: 1 },
      { key: 'minReboundPct', label: '自低点反弹 (路径 B)', type: 'number', min: 0.02, max: 0.15, step: 0.01, scale: 100, suffix: '%' },
      { key: 'buyAdjacentDays', label: '与「买」相邻排除', type: 'int', min: 0, max: 5, step: 1, suffix: '日' },
      { key: 'minGap', label: '同股重复标弹间隔', type: 'int', min: 3, max: 15, step: 1, suffix: '日' },
      { key: 'confirmDays', label: 'MA20 确认天数', type: 'int', min: 1, max: 4, step: 1, suffix: '日' },
      { key: 'maxUpperWickRatio', label: '上影线占振幅上限', type: 'number', min: 0.3, max: 0.8, step: 0.05, scale: 100, suffix: '%' },
      { key: 'screenMaxRsi', label: '列表筛选排除 RSI', type: 'number', min: 50, max: 75, step: 1 },
    ],
  },
  {
    key: 'buy',
    title: '买',
    fields: [
      { key: 'minBodyPct', label: '阳实体下限', type: 'number', min: 0.005, max: 0.05, step: 0.001, scale: 100, suffix: '%' },
    ],
  },
]

/** 设置项悬停说明（field.hint 优先） */
export const SIGNAL_FIELD_HINTS = {
  common: {
    rsiPeriod: '计算 RSI 的周期，默认 14。',
    iceThreshold: 'RSI 下穿该值标「冰」，上穿且近端有冰可标「买」。',
    overbought: 'RSI 在该值上方停留后回落，用于触发「止」。',
    lookback: '出「买」时，向前几天内须出现过「冰」。',
    maPeriod: '判定「减」所用的均线周期，默认 20。',
    volPeriod: '计算近均量周期，供「强」「突」放量判断。',
    requireIndexBull: '标「强」时要求上证指数收盘在 MA20 之上。',
  },
  sell: {
    reduceConfirmDays: '连续 N 日收盘低于 MA20 标减；1 = 首次跌破当日。',
    breakEntryLowEnablesReduce: '收盘首次跌破最近买点当日低点时也标减。',
    breakEntryLowLookback: '查找「最近买点」的回溯交易日数。',
    reduceMinGap: '同一只股票两次「减」之间最少间隔。',
    reduceEarlyEnabled: '开启后可在破 MA20 前按走弱规则提前标减。',
    reduceEarlyMa5Days: '连续 N 日收在 MA5 下，且仍在 MA20 附近时提前标减。',
    reduceEarlyMinDropPct: '前一日在 MA20 上，当日阴线跌幅达该比例时标减。',
    reduceEarlyBreakLowDays: '跌破近 N 日最低收盘/低点，且前一日在 MA20 上时标减。',
    takeProfitMinOverboughtDays: 'RSI 须在超买区连续停留 N 日后下穿才标止。',
    takeProfitMinGap: '同一只股票两次「止」之间最少间隔。',
    sellProfitGateEnabled: '关闭后止仅按 RSI 规则，不检查浮盈与回撤。',
    takeProfitRequirePullbackFromPeak: '关闭后不要求自阶段高点已有回撤。',
    takeProfitMinGainPct: '相对最近买点浮盈不足该比例不标止。',
    takeProfitMinPullbackFromPeakPct: '自买点以来最高收盘回撤不足则不标止。',
    protectAfterTrend: '标「趋」后 N 日内默认不标止/减；0 = 关闭。',
    protectAfterStrong: '标「强」后 N 日内默认不标止/减；0 = 关闭。',
    protectAfterBreakout: '标「突」后 N 日内默认不标止/减；0 = 关闭。',
    protectAfterReversal: '标「转」后 N 日内默认不标止/减；0 = 关闭。',
    protectAfterBuy: '标「买」后 N 日内默认不标止/减；0 = 关闭。',
    protectAfterRebound: '标「弹」后 N 日内默认不标止/减；0 = 关闭。',
  },
  strong: {
    volMult: '买点日成交量须 ≥ 近均量 × 该倍数。',
    confirmDays: '标「强」前须连续站稳的交易日数（不含买点日）。',
    iceMinGap: '出「强」与上一次「冰」之间最少间隔。',
    requireStockAboveMa20: '标「强」时个股须收在 MA20 之上。',
    requireMa20Rising: '标「强」时 MA20 须走平或向上。',
  },
  trend: {
    rsiMin: '标「趋」时 RSI 下限。',
    rsiMax: '标「趋」时 RSI 上限。',
    touchMaPct: '最低价触及 MA5/MA10 的容差比例。',
    maxCloseAboveMa5Pct: '收盘相对 MA5 涨幅超过该值不标趋；0 = 不限制。',
    minUpBars: '标趋前近端须有 N 日收在 MA20 上。',
    minGap: '同股两次「趋」之间最少间隔。',
    requireNextDayYang:
      '开启：回踩日条件满足后，须下一交易日收阳（收盘>开盘）才在次日标「趋」；关闭：保持回踩收阳当日即标。',
    chopFilterEnabled: '均线粘合、近端有止/减、横盘时不标趋。',
    minMaSpreadPct: 'MA5 须高于 MA10 至少该比例，否则视为粘合。',
    chopSellLookback: '近 N 日内有止/减则不标趋；0 = 不检查。',
    chopRangeDays: '判断横盘所用的近端交易日数。',
    chopMaxRangePct: '近 N 日振幅低于该值视为横盘，不标趋。',
  },
  reversal: {
    enabled: '关闭后不再标「转」势信号。',
    crossMaPeriod: '收盘上穿该周期均线且前段多数在均线下时标转。',
    minBodyPct: '转势日阳实体（相对开盘）最低比例。',
    minGap: '同股两次「转」之间最少间隔。',
    rsiMin: '标「转」时 RSI 下限。',
    rsiMax: '标「转」时 RSI 上限。',
    requireMa20FlatOrUp: 'MA20 须较对比日不走弱。',
    ma20LookbackDays: '与 N 日前 MA20 对比判断是否走平/向上。',
    requireCloseAboveMa5: '转势日收盘须高于 MA5。',
    requireMa5AboveMa10: '转势日须 MA5 > MA10。',
    recentBelowDays: '上穿前 N 日须在均线下。',
    recentBelowMinRatio: '上穿前 N 日内收在均线下天数占比下限。',
  },
  breakout: {
    boxPeriod: '识别箱体整理所用的观察周期。',
    maxRangePct: '箱体内最高价与最低价振幅上限。',
    volMult: '突破日成交量须 ≥ 近均量 × 该倍数。',
    minGap: '同股两次「突」之间最少间隔。',
    breakBuffer: '收盘须超过箱体上沿再加该缓冲比例。',
    confirmDays: '突破后须连续 N 日收盘站稳箱顶才标突。',
    maxUpperWickRatio: '突破日上影线占 K 线振幅上限，过长视为假突破。',
  },
  rebound: {
    sellLookback: '向前查找「止/减」的回溯窗口。',
    minDaysAfterSell: '卖信号后至少间隔 N 日才开始计弹。',
    minBodyPct: '标弹日阳实体最低比例。',
    maxRsi: '标弹日 RSI 须不超过该值。',
    minReboundPct: '路径 B：自卖后低点反弹幅度下限。',
    buyAdjacentDays: '与「买」相邻 N 日内不标弹。',
    minGap: '同股两次「弹」之间最少间隔。',
    confirmDays: '连续 N 日收盘 ≥ MA20 后第 N 日标弹。',
    maxUpperWickRatio: '弹信号日上影线占振幅上限。',
    screenMaxRsi: '列表筛选时 RSI 超过该值排除弹信号。',
  },
  buy: {
    minBodyPct: '出「买」时阳实体（相对开盘）最低比例。',
  },
}

export function getSignalFieldHint(sectionKey, field) {
  if (!field) return ''
  if (field.hint) return field.hint
  return SIGNAL_FIELD_HINTS[sectionKey]?.[field.key] || ''
}

function deepClone(obj) {
  return JSON.parse(JSON.stringify(obj))
}

function mergeSignalStrategySettings(raw) {
  const base = cloneDefaultSignalSettingsCore()
  if (!raw || typeof raw !== 'object') return base
  for (const section of SIGNAL_PARAM_SECTIONS) {
    const key = section.key
    if (!raw[key] || typeof raw[key] !== 'object') continue
    const values = { ...raw[key] }
    if (key === 'common') {
      delete values.recentBuyDays
      delete values.recentSellDays
    }
    base[key] = { ...base[key], ...values }
  }
  return base
}

/** Phase17.4：语义别名 signalPresets ≡ screenStrategies（兼容旧配置） */
function resolveRawPresetList(raw) {
  if (!raw || typeof raw !== 'object') return []
  if (Array.isArray(raw.signalPresets) && raw.signalPresets.length) return raw.signalPresets
  if (Array.isArray(raw.screenStrategies) && raw.screenStrategies.length) return raw.screenStrategies
  return []
}

function resolveRawActivePresetId(raw, fallback) {
  if (!raw || typeof raw !== 'object') return fallback
  const fromNew = String(raw.activeSignalPresetId || '').trim()
  if (fromNew) return fromNew
  const fromOld = String(raw.activeScreenStrategyId || '').trim()
  if (fromOld) return fromOld
  return fallback
}

/** 内存模型同时挂旧键与新别名，读路径统一走 screenStrategies 字段 */
function mirrorSignalPresetAliases(base) {
  if (!base || typeof base !== 'object') return base
  base.signalPresets = base.screenStrategies
  base.activeSignalPresetId = base.activeScreenStrategyId
  return base
}

export function cloneDefaultSignalSettings() {
  const base = deepClone(DEFAULT_SIGNAL_SETTINGS)
  base.activeScreenStrategyId = DEFAULT_SCREEN_STRATEGY_ID
  base.screenStrategies = [
    {
      id: DEFAULT_SCREEN_STRATEGY_ID,
      name: DEFAULT_SCREEN_STRATEGY_NAME,
      settings: cloneDefaultSignalSettingsCore(),
    },
  ]
  return mirrorSignalPresetAliases(base)
}

export function cloneDefaultSignalStrategySettings() {
  return cloneDefaultSignalSettingsCore()
}

export function mergeSignalSettings(raw) {
  const base = cloneDefaultSignalSettings()
  if (!raw || typeof raw !== 'object') return base
  if (raw.automation) base.automation = mergeQuantAutomation(raw.automation)
  if (raw.display) base.display = { ...base.display, ...raw.display }
  if (raw.common && typeof raw.common === 'object') {
    if (raw.display?.recentBuyDays == null && raw.common.recentBuyDays != null) {
      base.display.recentBuyDays = raw.common.recentBuyDays
    }
    if (raw.display?.recentSellDays == null && raw.common.recentSellDays != null) {
      base.display.recentSellDays = raw.common.recentSellDays
    }
  }
  for (const section of SIGNAL_PARAM_SECTIONS) {
    const key = section.key
    if (!raw[key] || typeof raw[key] !== 'object') continue
    base[key] = { ...base[key], ...raw[key] }
  }
  base.activeScreenStrategyId = resolveRawActivePresetId(
    raw,
    base.activeScreenStrategyId || DEFAULT_SCREEN_STRATEGY_ID,
  )
  const rawStrategies = resolveRawPresetList(raw)
  const strategies = rawStrategies
    .map((item, index) => {
      const id = String(item?.id || '').trim() || `strategy-${index + 1}`
      const name = String(item?.name || '').trim() || `参数预设 ${index + 1}`
      const settings = mergeSignalStrategySettings(item?.settings || {})
      return { id, name, settings }
    })
    .filter((item) => item.id && item.name)
  if (strategies.length) {
    base.screenStrategies = strategies
    if (!strategies.some((item) => item.id === base.activeScreenStrategyId)) {
      base.activeScreenStrategyId = strategies[0].id
    }
  } else {
    base.screenStrategies = [
      {
        id: DEFAULT_SCREEN_STRATEGY_ID,
        name: DEFAULT_SCREEN_STRATEGY_NAME,
        settings: mergeSignalStrategySettings(base),
      },
    ]
    base.activeScreenStrategyId = DEFAULT_SCREEN_STRATEGY_ID
  }
  return mirrorSignalPresetAliases(base)
}

export function parseSignalParams(raw) {
  if (!raw) return cloneDefaultSignalSettings()
  try {
    const parsed = typeof raw === 'string' ? JSON.parse(raw) : raw
    return mergeSignalSettings(parsed)
  } catch {
    return cloneDefaultSignalSettings()
  }
}

export function serializeSignalParams(settings) {
  // 双写 signalPresets + screenStrategies，旧端仍可读
  return JSON.stringify(mirrorSignalPresetAliases(mergeSignalSettings(settings)))
}

export function extractSignalStrategySettings(settings) {
  return mergeSignalStrategySettings(settings)
}

export function getScreenStrategies(settings) {
  return mergeSignalSettings(settings).screenStrategies
}

/** Phase17.4 语义别名：Signal 参数预设列表 */
export function getSignalPresets(settings) {
  return getScreenStrategies(settings)
}

export function getActiveScreenStrategyId(settings) {
  return mergeSignalSettings(settings).activeScreenStrategyId
}

export function getActiveSignalPresetId(settings) {
  return getActiveScreenStrategyId(settings)
}

export function getActiveScreenStrategy(settings) {
  const s = mergeSignalSettings(settings)
  return s.screenStrategies.find((item) => item.id === s.activeScreenStrategyId) || s.screenStrategies[0]
}

export function getScreenStrategyById(settings, strategyId) {
  const s = mergeSignalSettings(settings)
  return s.screenStrategies.find((item) => item.id === strategyId) || s.screenStrategies[0]
}

export function getActiveScreenStrategySettings(settings) {
  return getActiveScreenStrategy(settings)?.settings || extractSignalStrategySettings(settings)
}

export function getScreenStrategySettingsById(settings, strategyId) {
  return getScreenStrategyById(settings, strategyId)?.settings || extractSignalStrategySettings(settings)
}

export function setActiveScreenStrategy(settings, strategyId) {
  const s = mergeSignalSettings(settings)
  if (s.screenStrategies.some((item) => item.id === strategyId)) {
    s.activeScreenStrategyId = strategyId
  }
  return mirrorSignalPresetAliases(s)
}

/** 转为 computeFullSignals / summarizeBuySignal 的 options */
export function buildSignalOptions(settings) {
  const s = mergeSignalSettings(settings)
  const { common, buy, strong, trend, reversal, breakout, rebound, sell } = s
  const legacyProtect =
    sell?.protectAfterEntryDays ??
    common?.sellProtectAfterEntryDays ??
    DEFAULT_SIGNAL_SETTINGS.sell.protectAfterEntryDays
  return {
    rsiPeriod: common.rsiPeriod,
    iceThreshold: common.iceThreshold,
    overbought: common.overbought,
    lookback: common.lookback,
    maPeriod: common.maPeriod,
    volPeriod: common.volPeriod,
    recentBuyDays: s.display?.recentBuyDays ?? common.recentBuyDays ?? DEFAULT_SIGNAL_SETTINGS.display.recentBuyDays,
    recentSellDays: s.display?.recentSellDays ?? common.recentSellDays ?? DEFAULT_SIGNAL_SETTINGS.display.recentSellDays,
    requireIndexBull: common.requireIndexBull,
    reduceConfirmDays: sell.reduceConfirmDays,
    reduceEarlyEnabled: sell.reduceEarlyEnabled !== false,
    reduceEarlyMa5Days: sell.reduceEarlyMa5Days,
    reduceEarlyMinDropPct: sell.reduceEarlyMinDropPct,
    reduceEarlyBreakLowDays: sell.reduceEarlyBreakLowDays,
    reduceEarlyUptrendLookback: sell.reduceEarlyUptrendLookback,
    reduceEarlyRequireUptrend: sell.reduceEarlyRequireUptrend !== false,
    takeProfitMinOverboughtDays: sell.takeProfitMinOverboughtDays,
    sellBreakEntryLowEnablesReduce: sell.breakEntryLowEnablesReduce,
    sellBreakEntryLowLookback: sell.breakEntryLowLookback,
    sellProtectAfterBuy: sell.protectAfterBuy ?? legacyProtect,
    sellProtectAfterStrong: sell.protectAfterStrong ?? legacyProtect,
    sellProtectAfterTrend: sell.protectAfterTrend ?? legacyProtect,
    sellProtectAfterBreakout: sell.protectAfterBreakout ?? legacyProtect,
    sellProtectAfterRebound: sell.protectAfterRebound ?? legacyProtect,
    sellProtectAfterReversal: sell.protectAfterReversal ?? legacyProtect,
    sellProtectAfterEntryDays: legacyProtect,
    sellReduceMinGap: sell.reduceMinGap,
    sellTakeProfitMinGap: sell.takeProfitMinGap,
    sellProfitGateEnabled: sell.sellProfitGateEnabled !== false,
    takeProfitMinGainPct: sell.takeProfitMinGainPct,
    takeProfitMinPullbackFromPeakPct: sell.takeProfitMinPullbackFromPeakPct,
    takeProfitRequirePullbackFromPeak: sell.takeProfitRequirePullbackFromPeak !== false,
    buyMinBodyPct: buy.minBodyPct,
    strictVolMult: strong.volMult,
    strongConfirmDays: strong.confirmDays,
    iceMinGap: strong.iceMinGap,
    requireStockAboveMa20: strong.requireStockAboveMa20,
    requireMa20Rising: strong.requireMa20Rising,
    trendRsiMin: trend.rsiMin,
    trendRsiMax: trend.rsiMax,
    trendTouchMaPct: trend.touchMaPct,
    trendMaxCloseAboveMa5Pct: trend.maxCloseAboveMa5Pct,
    trendMinUpBars: trend.minUpBars,
    trendMinGap: trend.minGap,
    trendRequireNextDayYang: trend.requireNextDayYang === true,
    trendChopFilterEnabled: trend.chopFilterEnabled !== false,
    trendMinMaSpreadPct: trend.minMaSpreadPct,
    trendChopSellLookback: trend.chopSellLookback,
    trendChopRangeDays: trend.chopRangeDays,
    trendChopMaxRangePct: trend.chopMaxRangePct,
    reversalEnabled: reversal.enabled !== false,
    reversalCrossMaPeriod: reversal.crossMaPeriod,
    reversalMinBodyPct: reversal.minBodyPct,
    reversalRsiMin: reversal.rsiMin,
    reversalRsiMax: reversal.rsiMax,
    reversalRequireMa20FlatOrUp: reversal.requireMa20FlatOrUp !== false,
    reversalMa20LookbackDays: reversal.ma20LookbackDays,
    reversalRequireMa5AboveMa10: reversal.requireMa5AboveMa10 === true,
    reversalRequireCloseAboveMa5: reversal.requireCloseAboveMa5 !== false,
    reversalRecentBelowDays: reversal.recentBelowDays,
    reversalRecentBelowMinRatio: reversal.recentBelowMinRatio,
    reversalMinGap: reversal.minGap,
    boxPeriod: breakout.boxPeriod,
    maxRangePct: breakout.maxRangePct,
    volMult: breakout.volMult,
    minGap: breakout.minGap,
    breakBuffer: breakout.breakBuffer,
    confirmDays: breakout.confirmDays,
    breakoutConfirmDays: breakout.confirmDays,
    maxUpperWickRatio: breakout.maxUpperWickRatio,
    reboundSellLookback: rebound.sellLookback,
    reboundMinDaysAfterSell: rebound.minDaysAfterSell,
    reboundMinBodyPct: rebound.minBodyPct,
    reboundMaxRsi: rebound.maxRsi,
    reboundMinPct: rebound.minReboundPct,
    reboundBuyAdjacentDays: rebound.buyAdjacentDays,
    reboundMinGap: rebound.minGap,
    reboundConfirmDays: rebound.confirmDays,
    reboundMaxUpperWickRatio: rebound.maxUpperWickRatio,
  }
}

export function getReboundScreenMaxRsi(settings) {
  const s = mergeSignalSettings(settings)
  return s.rebound.screenMaxRsi ?? DEFAULT_SIGNAL_SETTINGS.rebound.screenMaxRsi
}

export function getFollowDateGroupSettings(settings) {
  const s = mergeSignalSettings(settings)
  const raw = Number(s.display?.dateGroupRetainDays)
  return {
    enabled: s.display?.followDateGroupEnabled !== false,
    retainDays: Number.isFinite(raw) ? Math.max(7, Math.min(180, Math.round(raw))) : 30,
  }
}

/** 自选股票数量上限（默认 100，最大 500） */
export function getMaxFollowCount(settings) {
  const s = mergeSignalSettings(settings)
  const raw = Number(s.display?.maxFollowCount)
  if (!Number.isFinite(raw) || raw <= 0) return 100
  return Math.max(1, Math.min(500, Math.round(raw)))
}

export function restoreSignalSection(settings, sectionKey) {
  const next = mergeSignalSettings(settings)
  if (sectionKey === 'display' && DEFAULT_SIGNAL_SETTINGS.display) {
    next.display = JSON.parse(JSON.stringify(DEFAULT_SIGNAL_SETTINGS.display))
    return next
  }
  if (DEFAULT_SIGNAL_SETTINGS[sectionKey]) {
    next[sectionKey] = deepClone(DEFAULT_SIGNAL_SETTINGS[sectionKey])
  }
  return next
}
