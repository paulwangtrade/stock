/**
 * 每日精进 — static, curated discipline tips.
 * No runtime LLM. One tip per Asia/Shanghai calendar day.
 * Optional「下一条」only walks a same-day pool; the next calendar day
 * keeps its own stable assignment.
 * UI must show DAILY_DISCIPLINE_DISCLAIMER with every tip.
 */

import { shanghaiDate } from './planContext.js'

export const DAILY_DISCIPLINE_STORAGE_KEY = 'go-stock.dailyDiscipline'
export const DAILY_DISCIPLINE_DISCLAIMER =
  '非投资建议，不是交易信号，也不构成买卖指令。'
export const DAILY_DISCIPLINE_POOL_SIZE = 3
/** Prefer tips not shown in this many calendar days. */
export const DAILY_DISCIPLINE_PREFER_DAYS = 14
/** Shown inside this window is the last resort (least-recently-shown). */
export const DAILY_DISCIPLINE_SOFT_DAYS = 7
export const DAILY_DISCIPLINE_STATE_VERSION = 1

export const TIP_CATEGORIES = Object.freeze([
  '心态',
  '纪律',
  '仓位',
  '失效条件',
  '观察≠下单',
  '止损/止盈纪律',
  '勿用票数当买点',
  '研究台定位',
])

/** @typedef {{ id: string, category: string, title: string, summary: string, body: string }} DailyTip */

/** @type {readonly DailyTip[]} */
export const DAILY_TIPS = Object.freeze([
  {
    id: 'dd-01',
    category: '心态',
    title: '先慢半拍',
    summary: '行情冲过来时，慢半拍往往比追上去更稳。',
    body: '心跳变快的时候，先把眼睛从闪动的数字上移开。你要核对的是事先写好的条件，不是证明自己反应够快。慢，是纪律的一部分。',
  },
  {
    id: 'dd-02',
    category: '心态',
    title: '不追回本',
    summary: '亏损之后最危险的动作，是立刻找一笔「扳回来」。',
    body: '回本不是下一笔的目标。先离开键盘几分钟，确认新的想法仍然符合原规则。用更大的仓位去追回损失，通常只是把错误放大。不追回本，也不追恢复。',
  },
  {
    id: 'dd-03',
    category: '心态',
    title: '做对了也先收一收',
    summary: '一笔顺利，只说明这次条件碰巧对齐，不说明方法已经升级。',
    body: '盈利后容易觉得自己看懂了。把这次写成：哪些条件成立，哪些只是运气。下次仍按同一张清单检查，而不是加大胆量。',
  },
  {
    id: 'dd-04',
    category: '心态',
    title: '空着也算一种持仓',
    summary: '今天没有动作，不等于今天没有纪律。',
    body: '空仓是在等条件，不是在浪费行情。失效条件没解除、观察还没写成计划时，不动就是正确执行。',
  },
  {
    id: 'dd-05',
    category: '心态',
    title: '别人的成交不是你的时钟',
    summary: '群里已经买到，不构成你现在必须跟上的理由。',
    body: '你看不到对方的仓位、成本和失效条件。只问自己：观察完成了吗？计划写了吗？没有就继续看，别把别人的成交当成你的信号。',
  },
  {
    id: 'dd-06',
    category: '心态',
    title: '烦的时候先别动手',
    summary: '烦躁、委屈、想证明自己，都不是入场理由。',
    body: '情绪上来时，把「想做」改写成一句观察。等这句话能平静地读出来，再决定要不要进入计划。交易台不是情绪出口。',
  },
  {
    id: 'dd-07',
    category: '心态',
    title: '连错之后缩小',
    summary: '连续不顺时，先把手数降下来，而不是加快下一次。',
    body: '连错往往说明市场和你的假设暂时不合。缩小观察范围、减少新开仓，比「再试一把」更接近纪律。不要为了追回前面的亏损而加密交易。',
  },
  {
    id: 'dd-08',
    category: '纪律',
    title: '先写下来再动手',
    summary: '说不清为什么做，就先不要做。',
    body: '用一句话写下：做什么、为什么是现在、错了看哪里。写不出来，就还停在观察。纪律是把冲动变成可以复查的句子。',
  },
  {
    id: 'dd-09',
    category: '纪律',
    title: '计划外的冲动先搁置',
    summary: '临时冒出来的标的，默认今天不做。',
    body: '新想法可以记进研究台，但不要挤进已经写好的交易计划。计划外开仓，是最常见的纪律泄漏口。',
  },
  {
    id: 'dd-10',
    category: '纪律',
    title: '收盘再问三个问题',
    summary: '今天做的是否都在计划里，没做的是否本该做。',
    body: '三个问题就够：哪些按计划执行了，哪些是临时起意，哪些观察明天仍有效。复盘是为了下次更稳，不是为了责怪自己。',
  },
  {
    id: 'dd-11',
    category: '纪律',
    title: '规则不要盘中改',
    summary: '盘中改规则，通常是在给冲动找理由。',
    body: '仓位上限、止损位置、失效条件，开盘前就定好。盘中只执行，不重新谈判。想改，写到明天的研究笔记里。',
  },
  {
    id: 'dd-12',
    category: '纪律',
    title: '信号不是委托',
    summary: '信号亮了，只说明可以开始核对，不说明可以下单。',
    body: '把信号当成门铃，而不是成交键。核对失效条件、仓位和计划是否允许，三样都过了才谈得上下单。信号 ≠ 订单。',
  },
  {
    id: 'dd-13',
    category: '纪律',
    title: '观察不是交易计划',
    summary: '研究台上的笔记，不会自动变成可以执行的计划。',
    body: '观察回答「我看到了什么」。交易计划回答「我允许自己做什么、错了怎么办」。两件事分开写。观察 ≠ TradePlan。',
  },
  {
    id: 'dd-14',
    category: '纪律',
    title: '清单之外先说不',
    summary: '今天只处理清单上的名字，其余一律先观察。',
    body: '清单是你给自己的边界。临时跳出来的强势股，先放进观察，不进今天的计划。说「不」是纪律，不是错过。',
  },
  {
    id: 'dd-15',
    category: '纪律',
    title: '下单前读一遍失效条件',
    summary: '手指放到确认键之前，先把「什么情况算错」读出声。',
    body: '读得出来，说明这笔还有边界。读不出来，就回到观察。没有失效条件的决定，不该出现在交易台上。',
  },
  {
    id: 'dd-16',
    category: '仓位',
    title: '单票不要吃掉整份胆量',
    summary: '再喜欢的标的，也只占你事先写好的那一格仓位。',
    body: '仓位是风险预算，不是信心温度计。信心越高越想加仓时，正好停在上限。超出上限的部分，留在观察里。',
  },
  {
    id: 'dd-17',
    category: '仓位',
    title: '第一笔只是试探',
    summary: '计划里的第一笔，用来验证条件，不是用来一次性打满。',
    body: '试探仓小一点，错了离场的成本也小。后续加仓必须仍满足原来的条件，而不是因为「已经做了所以要做够」。',
  },
  {
    id: 'dd-18',
    category: '仓位',
    title: '先看总仓，再看这一笔',
    summary: '单票看起来不重，加在一起可能已经超了。',
    body: '动手前先看现金和总仓位。总仓到线，新的想法就只能观察，不能再开。保护的是整个账户，不是某一只票的故事。',
  },
  {
    id: 'dd-19',
    category: '仓位',
    title: '不对着亏损加仓',
    summary: '价格更低，不等于理由更充分。',
    body: '摊平是新的一笔决定，必须重新满足入场条件，而不是为了拉低成本。若失效条件已经触发，该减的是仓，不是再补一笔。不要用加仓去追回亏损。',
  },
  {
    id: 'dd-20',
    category: '仓位',
    title: '留一点现金给计划',
    summary: '把现金打光，等于把明天的纪律也花掉了。',
    body: '预留现金，是为了计划内的安排，不是为了看见波动就用掉。没有现金时，研究可以继续，新开仓先停。',
  },
  {
    id: 'dd-21',
    category: '仓位',
    title: '看起来不同，风险可能一样',
    summary: '会一起跌的标的，仓位要当成一笔来看。',
    body: '同板块、同逻辑的持仓会一起失效。数仓位时按主题合并，而不是按股票只数安慰自己「已经很分散」。',
  },
  {
    id: 'dd-22',
    category: '失效条件',
    title: '错了看哪里，先写好',
    summary: '进场之前就写下一句：出现什么，这笔假设作废。',
    body: '失效条件要具体：价格、事实或天数，至少有一个能核对。含糊的「看情况」等于没有条件。',
  },
  {
    id: 'dd-23',
    category: '失效条件',
    title: '条件到了就离开',
    summary: '失效不是再讨论一次的邀请，而是事先答应好的离场。',
    body: '真的触发时，人会想再等一根K线。这时执行原句，比临时找理由更重要。讨论留到收盘以后。',
  },
  {
    id: 'dd-24',
    category: '失效条件',
    title: '分清波动和假设破裂',
    summary: '正常回撤可以按计划拿着，假设破裂就要停。',
    body: '价格晃一下，不一定是失效。若你写下的理由已经被否定，价格还没跌很多也该重新评估。别用「还没亏多少」代替失效条件。',
  },
  {
    id: 'dd-25',
    category: '失效条件',
    title: '等到期了也是一种失效',
    summary: '计划里的等待天数用完，就当条件未满足。',
    body: '不是每一笔都要靠止损价结束。时间到了仍不走，说明观察没有兑现。到期离场或退回观察，都比无限期陪着更干净。',
  },
  {
    id: 'dd-26',
    category: '失效条件',
    title: '失效线不往下改',
    summary: '盘中把止损改远，等于临时取消了自己的承诺。',
    body: '可以在开盘前根据研究调整下一笔的条件，但不要在这笔已经危险时把线挪开。挪线常常是为了不承认错，而不是因为出现了新事实。',
  },
  {
    id: 'dd-27',
    category: '失效条件',
    title: '只剩故事时就降级',
    summary: '原来的几条理由只剩一条时，仓位不该还是满的。',
    body: '理由减少，风险预算也减少。降到试探仓或退回观察，比说服自己「剩下的那条更重要」更诚实。',
  },
  {
    id: 'dd-28',
    category: '观察≠下单',
    title: '研究台只负责看清楚',
    summary: '研究台用来记录观察，不负责催你成交。',
    body: '把研究台当成工作台：事实、疑问、失效条件。它不是下单入口。观察写完，仍然要单独决定要不要做成交易计划。',
  },
  {
    id: 'dd-29',
    category: '观察≠下单',
    title: '加自选不是买点',
    summary: '放进自选，只是方便明天继续看。',
    body: '自选列表变长很正常，长度不代表机会质量。从自选到计划，中间还差理由、仓位和失效条件。',
  },
  {
    id: 'dd-30',
    category: '观察≠下单',
    title: '看懂了也可以不做',
    summary: '理解一家公司，和做这一笔，是两件不同的事。',
    body: '研究可以很充分，但价格、位置、仓位或时机不对时，结论就是继续观察。看懂是为了少犯错，不是为了必须参与。',
  },
  {
    id: 'dd-31',
    category: '观察≠下单',
    title: '警报是提醒，不是指令',
    summary: '弹出一条异动，先打开笔记，而不是打开下单窗。',
    body: '异动说明发生了变化。变化要解释，解释要核对，核对通过才可能进入计划。警报 ≠ 订单。',
  },
  {
    id: 'dd-32',
    category: '观察≠下单',
    title: '分数高也不自动成交',
    summary: '排序靠前，只是今天优先看它，不是优先买它。',
    body: '分数帮助你分配注意力。注意力用完，若条件不齐，就停在观察。不要把排名当成买点。',
  },
  {
    id: 'dd-33',
    category: '观察≠下单',
    title: '没写进计划就还在看',
    summary: '只有写进交易计划的，才允许盘中执行。',
    body: '脑子里的「差不多可以」不算计划。计划里要有标的、仓位和失效条件。缺任何一项，今天就保持观察。观察 ≠ TradePlan。',
  },
  {
    id: 'dd-34',
    category: '观察≠下单',
    title: '多写观察，少开新仓',
    summary: '今天的产出可以是三句笔记，而不是三笔成交。',
    body: '成交次数不是努力程度。把看到的结构、风险和未解问题写下来，已经是完成了研究。新开仓只留给清单内、条件齐的那一笔。',
  },
  {
    id: 'dd-35',
    category: '观察≠下单',
    title: '进计划要过三道门',
    summary: '理由清楚、仓位写明、失效条件具体，三道都过才进计划。',
    body: '少一道就退回观察。这样计划会变短，但每一笔都知道自己在做什么。短计划比长愿望更可执行。',
  },
  {
    id: 'dd-36',
    category: '止损/止盈纪律',
    title: '止损是事先接受的费用',
    summary: '小亏是计划的一部分，不是执行失败。',
    body: '把止损看成进场时就同意支付的费用。该走就走，不要因为「再等一下也许回来」把费用变成大账单。也不要为了追回这笔小亏立刻再开一笔。',
  },
  {
    id: 'dd-37',
    category: '止损/止盈纪律',
    title: '盈利也按句子走',
    summary: '赚了不等于可以随便拿，或随便跑。',
    body: '事先写好：到哪里减一点，什么情况让利润继续。临时全走或临时死拿，都是盘中改规则。止盈纪律和止损一样，要在平静时写好。',
  },
  {
    id: 'dd-38',
    category: '止损/止盈纪律',
    title: '目标价不是誓言',
    summary: '写了目标价，仍要服从失效条件。',
    body: '没到目标价但理由没了，就离开。到了目标价但只是情绪想多拿，也按原计划减。价格目标服务于计划，不反过来指挥计划。',
  },
  {
    id: 'dd-39',
    category: '止损/止盈纪律',
    title: '保护利润要提前说好',
    summary: '浮盈回撤到哪一步就减，开盘前写进计划。',
    body: '盘中才发明「再让一点」，通常是舍不得。保护规则写得具体一点：回撤到哪、跌破哪条事先定的线。然后照做。',
  },
  {
    id: 'dd-40',
    category: '止损/止盈纪律',
    title: '先锁定一部分',
    summary: '计划允许的话，先减一点，剩下的按原失效条件拿。',
    body: '分批是为了让决策变轻，不是为了盘中纠结。减完把剩余仓位的失效条件再读一遍。不要减完又因为害怕踏空马上买回。',
  },
  {
    id: 'dd-41',
    category: '止损/止盈纪律',
    title: '止损之后先回到观察',
    summary: '刚离场的标的，默认今天不再开。',
    body: '马上买回，常常是不接受刚才的决定。除非原计划明确写了再入场条件并且已经满足，否则止损后的下一站是观察笔记，不是新订单。这不是追回本的入口。',
  },
  {
    id: 'dd-42',
    category: '勿用票数当买点',
    title: '十只观察不等于十个买点',
    summary: '列表很长，只说明你还在看，不说明你该买。',
    body: '用只数安慰自己「总有一只能成」，是把研究变成抽签。每只都要单独回答：为什么是现在，错了怎么办。答不上来就留在列表里。',
  },
  {
    id: 'dd-43',
    category: '勿用票数当买点',
    title: '涨的家数不是入场理由',
    summary: '今天很多票在涨，并不提高你这一笔的胜算。',
    body: '市场宽度可以帮你理解环境，但不能代替个股的计划和失效条件。不要因为「别人都在动」就降低自己的门槛。',
  },
  {
    id: 'dd-44',
    category: '勿用票数当买点',
    title: '自选越长越要挑',
    summary: '自选变多时，先删掉说不清理由的，而不是每个都买一点。',
    body: '维护列表是研究动作。留下仍值得跟踪的，其余归档。买点来自条件，不来自列表长度。',
  },
  {
    id: 'dd-45',
    category: '勿用票数当买点',
    title: '买很多只很小，仍可能没有计划',
    summary: '每只都买一点，看起来谨慎，其实可能是在回避选择。',
    body: '真正的分散要有不同的逻辑和各自的失效条件。若只是「都看看所以都买」，那是用票数代替判断。先观察，挑出写得清的再进计划。',
  },
  {
    id: 'dd-46',
    category: '勿用票数当买点',
    title: '信号条数不是仓位',
    summary: '同一只票亮了三次，仍然只是一只票、一个决定。',
    body: '不要把重复提醒累加成「更该买」。去重之后问：条件是否新成立。没有新事实，条数再多也不构成订单。信号 ≠ 下单。',
  },
  {
    id: 'dd-47',
    category: '研究台定位',
    title: '研究台不下达指令',
    summary: '在研究台把问题写清楚，就完成了它的工作。',
    body: '研究台的产出是笔记、对比和未决问题。交易台才执行已经写好的计划。不要在研究页面里把「感兴趣」点成成交。观察留在这里，订单不从这里出生。',
  },
  {
    id: 'dd-48',
    category: '研究台定位',
    title: '好的研究以问题结尾',
    summary: '写下还没回答的问题，比写下「可以买」更有用。',
    body: '例如：仓位是否已满、失效条件是否具体、是不是只是跟随别人。问题还在，就继续观察。研究台负责把问题照亮。',
  },
  {
    id: 'dd-49',
    category: '研究台定位',
    title: '笔记不会自动升级成计划',
    summary: '从研究台到交易计划，必须你亲手再确认一次。',
    body: '没有这道确认，再完整的笔记也只是观察。确认时检查仓位和失效条件。观察 ≠ TradePlan，这条边界留在两个页面之间。',
  },
  {
    id: 'dd-50',
    category: '研究台定位',
    title: '否决也是研究结果',
    summary: '今天决定不做，把原因写下来，研究就没有白做。',
    body: '「不做，因为失效条件还不清楚」是一条完整结论。下次打开同一标的，先读这条，避免重复被同一冲动带走。',
  },
  {
    id: 'dd-51',
    category: '研究台定位',
    title: '研究台用来对比，不只是收集',
    summary: '两只里写下谁的理由更完整，比两只都放进计划更有用。',
    body: '对比仓位占用、失效是否可执行、是不是同一类风险。对比完通常只留下少数观察，这是研究台该有的样子。',
  },
  {
    id: 'dd-52',
    category: '心态',
    title: '回撤不是开新仓的理由',
    summary: '账户回撤时，第一反应应是检查规则，而不是寻找补偿。',
    body: '补偿性交易最容易绕过计划和仓位。先问：若没有这笔回撤，我还会做吗？答案是否，就停在观察。不追回撤，不追恢复。',
  },
])

const YMD = /^(\d{4})-(\d{2})-(\d{2})$/

/**
 * @param {string} ymd
 * @returns {number|null} UTC midnight millis
 */
function ymdToUtc(ymd) {
  const m = YMD.exec(String(ymd || '').trim())
  if (!m) return null
  return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]))
}

/** Whole calendar days from `fromYmd` to `toYmd` (negative if from is later). */
export function calendarDaysBetween(fromYmd, toYmd) {
  const a = ymdToUtc(fromYmd)
  const b = ymdToUtc(toYmd)
  if (a == null || b == null) return 0
  return Math.round((b - a) / 86400000)
}

export function emptyDailyDisciplineState() {
  return {
    version: DAILY_DISCIPLINE_STATE_VERSION,
    history: [],
    assignment: null,
    dismissedDate: null,
  }
}

/**
 * @param {unknown[]} history
 * @returns {{ id: string, date: string }[]}
 */
export function compactHistory(history) {
  /** @type {Map<string, string>} */
  const latest = new Map()
  if (!Array.isArray(history)) return []
  for (const item of history) {
    if (!item || typeof item !== 'object') continue
    const id = String(item.id || '').trim()
    const date = String(item.date || '').trim()
    if (!id || !YMD.test(date)) continue
    const prev = latest.get(id)
    if (!prev || date > prev) latest.set(id, date)
  }
  return Array.from(latest, ([id, date]) => ({ id, date })).sort((a, b) => {
    if (a.date !== b.date) return a.date < b.date ? -1 : 1
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
  })
}

/**
 * @param {unknown} raw
 */
export function normalizeDailyDisciplineState(raw) {
  const base = emptyDailyDisciplineState()
  if (!raw || typeof raw !== 'object') return base
  const o = /** @type {Record<string, unknown>} */ (raw)
  const assignment = normalizeAssignment(o.assignment)
  const dismissedDate =
    typeof o.dismissedDate === 'string' && YMD.test(o.dismissedDate.trim())
      ? o.dismissedDate.trim()
      : null
  return {
    version: DAILY_DISCIPLINE_STATE_VERSION,
    history: compactHistory(/** @type {unknown[]} */ (o.history)),
    assignment,
    dismissedDate,
  }
}

function normalizeAssignment(raw) {
  if (!raw || typeof raw !== 'object') return null
  const o = /** @type {Record<string, unknown>} */ (raw)
  const date = typeof o.date === 'string' ? o.date.trim() : ''
  if (!YMD.test(date)) return null
  const pool = Array.isArray(o.pool)
    ? o.pool.map((id) => String(id || '').trim()).filter(Boolean)
    : []
  const index = Number.isFinite(Number(o.index)) ? Math.max(0, Math.floor(Number(o.index))) : 0
  return { date, pool, index }
}

/**
 * @param {Pick<Storage, 'getItem'>|null|undefined} storage
 */
export function readDailyDisciplineState(storage) {
  if (!storage || typeof storage.getItem !== 'function') return emptyDailyDisciplineState()
  try {
    const raw = storage.getItem(DAILY_DISCIPLINE_STORAGE_KEY)
    if (!raw) return emptyDailyDisciplineState()
    return normalizeDailyDisciplineState(JSON.parse(raw))
  } catch {
    return emptyDailyDisciplineState()
  }
}

/**
 * @param {ReturnType<typeof emptyDailyDisciplineState>} state
 * @param {Pick<Storage, 'setItem'>|null|undefined} storage
 */
export function writeDailyDisciplineState(state, storage) {
  if (!storage || typeof storage.setItem !== 'function') return
  try {
    const next = normalizeDailyDisciplineState(state)
    storage.setItem(DAILY_DISCIPLINE_STORAGE_KEY, JSON.stringify(next))
  } catch {
    /* private mode / quota: banner still works for this session */
  }
}

/**
 * Lower tier is more preferred.
 * 0 never shown, 1 not in the last `preferDays`, 2 in the soft window, 3 shown recently.
 * @param {readonly DailyTip[]} tips
 * @param {{ id: string, date: string }[]} history
 * @param {string} today
 */
export function rankDailyTips(tips, history, today, options = {}) {
  const preferDays = options.preferDays ?? DAILY_DISCIPLINE_PREFER_DAYS
  const softDays = options.softDays ?? DAILY_DISCIPLINE_SOFT_DAYS
  /** @type {Map<string, string>} */
  const last = new Map()
  for (const item of compactHistory(history)) {
    if (item.date > today) continue
    last.set(item.id, item.date)
  }
  const rows = (Array.isArray(tips) ? tips : []).map((tip) => {
    const shown = last.get(tip.id) || ''
    let tier = 0
    if (shown) {
      const age = calendarDaysBetween(shown, today)
      if (age >= preferDays) tier = 1
      else if (age >= softDays) tier = 2
      else tier = 3
    }
    return { id: tip.id, tier, shown: shown || '0000-00-00' }
  })
  rows.sort((a, b) => {
    if (a.tier !== b.tier) return a.tier - b.tier
    if (a.shown !== b.shown) return a.shown < b.shown ? -1 : 1
    return a.id < b.id ? -1 : a.id > b.id ? 1 : 0
  })
  return rows
}

function tipIds(tips) {
  return new Set((Array.isArray(tips) ? tips : []).map((tip) => tip.id))
}

function assignmentUsable(assignment, today, tips) {
  if (!assignment || assignment.date !== today) return false
  if (!Array.isArray(assignment.pool) || assignment.pool.length === 0) return false
  const ids = tipIds(tips)
  return assignment.pool.every((id) => ids.has(id))
}

function clampIndex(index, length) {
  if (length <= 0) return 0
  const n = Number.isFinite(Number(index)) ? Math.floor(Number(index)) : 0
  if (n < 0) return 0
  if (n >= length) return 0
  return n
}

export function recordShown(history, id, date) {
  const next = compactHistory(history).filter((item) => item.id !== id)
  next.push({ id, date })
  return compactHistory(next)
}

/**
 * Same calendar day keeps the stored pool and index.
 * A new day builds a fresh pool: unseen / outside 14 days first, else least-recently-shown.
 * @param {ReturnType<typeof emptyDailyDisciplineState>} state
 * @param {string} today
 * @param {readonly DailyTip[]} tips
 */
export function ensureDailyAssignment(state, today, tips, options = {}) {
  const normalized = normalizeDailyDisciplineState(state)
  if (!YMD.test(String(today || ''))) return normalized
  const list = Array.isArray(tips) ? tips : []
  if (assignmentUsable(normalized.assignment, today, list)) {
    const index = clampIndex(normalized.assignment.index, normalized.assignment.pool.length)
    if (index === normalized.assignment.index) return normalized
    return {
      ...normalized,
      assignment: { ...normalized.assignment, index },
    }
  }
  const poolSize = options.poolSize ?? DAILY_DISCIPLINE_POOL_SIZE
  const ranked = rankDailyTips(list, normalized.history, today, options)
  const pool = ranked.slice(0, Math.max(0, poolSize)).map((row) => row.id)
  if (!pool.length) {
    return { ...normalized, assignment: { date: today, pool: [], index: 0 } }
  }
  return {
    ...normalized,
    history: recordShown(normalized.history, pool[0], today),
    assignment: { date: today, pool, index: 0 },
  }
}

function resolveOptions(options = {}) {
  const storage =
    options.storage !== undefined
      ? options.storage
      : typeof localStorage !== 'undefined'
        ? localStorage
        : null
  const now = options.now instanceof Date ? options.now : new Date()
  const today = typeof options.today === 'string' && options.today ? options.today : shanghaiDate(now)
  const tips = options.tips || DAILY_TIPS
  return { storage, now, today, tips, poolSize: options.poolSize, preferDays: options.preferDays, softDays: options.softDays }
}

/**
 * @param {ReturnType<typeof emptyDailyDisciplineState>} state
 * @param {string} today
 * @param {readonly DailyTip[]} tips
 */
export function toDailyDisciplineView(state, today, tips) {
  const byId = new Map((Array.isArray(tips) ? tips : []).map((tip) => [tip.id, tip]))
  const pool = state.assignment && state.assignment.date === today ? state.assignment.pool : []
  const index = clampIndex(state.assignment?.index, pool.length)
  const tip = byId.get(pool[index]) || null
  const dismissed = state.dismissedDate === today
  return {
    date: today,
    visible: Boolean(tip) && !dismissed,
    tip,
    index,
    poolSize: pool.length,
    canAdvance: pool.length > 1,
    positionLabel: pool.length > 1 ? `${index + 1}/${pool.length}` : '',
    disclaimer: DAILY_DISCIPLINE_DISCLAIMER,
  }
}

/** Load (and persist) today's stable tip. */
export function presentDailyDiscipline(options = {}) {
  const { storage, today, tips, poolSize, preferDays, softDays } = resolveOptions(options)
  const state = ensureDailyAssignment(readDailyDisciplineState(storage), today, tips, {
    poolSize,
    preferDays,
    softDays,
  })
  writeDailyDisciplineState(state, storage)
  return toDailyDisciplineView(state, today, tips)
}

/** Advance within today's pool only. Does not pull a new day or a new pool. */
export function advanceDailyDiscipline(options = {}) {
  const { storage, today, tips, poolSize, preferDays, softDays } = resolveOptions(options)
  let state = ensureDailyAssignment(readDailyDisciplineState(storage), today, tips, {
    poolSize,
    preferDays,
    softDays,
  })
  const pool = state.assignment?.pool || []
  if (pool.length > 1 && state.assignment?.date === today) {
    const index = (clampIndex(state.assignment.index, pool.length) + 1) % pool.length
    const id = pool[index]
    state = {
      ...state,
      history: recordShown(state.history, id, today),
      assignment: { ...state.assignment, index },
    }
  }
  writeDailyDisciplineState(state, storage)
  return toDailyDisciplineView(state, today, tips)
}

/** Hide the banner until the next Shanghai calendar day. */
export function dismissDailyDiscipline(options = {}) {
  const { storage, today, tips, poolSize, preferDays, softDays } = resolveOptions(options)
  const ensured = ensureDailyAssignment(readDailyDisciplineState(storage), today, tips, {
    poolSize,
    preferDays,
    softDays,
  })
  const state = { ...ensured, dismissedDate: today }
  writeDailyDisciplineState(state, storage)
  return toDailyDisciplineView(state, today, tips)
}
