var SignalScanBatch = (() => {
  var __defProp = Object.defineProperty;
  var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
  var __getOwnPropNames = Object.getOwnPropertyNames;
  var __hasOwnProp = Object.prototype.hasOwnProperty;
  var __export = (target, all) => {
    for (var name in all)
      __defProp(target, name, { get: all[name], enumerable: true });
  };
  var __copyProps = (to, from, except, desc) => {
    if (from && typeof from === "object" || typeof from === "function") {
      for (let key of __getOwnPropNames(from))
        if (!__hasOwnProp.call(to, key) && key !== except)
          __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
    }
    return to;
  };
  var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

  // ../scripts/scansignals/scan-batch.ts
  var scan_batch_exports = {};
  __export(scan_batch_exports, {
    runSignalScanBatch: () => runSignalScanBatch
  });

  // src/utils/sellPositionRatio.js
  function clamp(n, min, max) {
    return Math.max(min, Math.min(max, n));
  }
  function roundPct01(p) {
    return Math.round(p * 20) / 20;
  }
  function volMa(volumes, period, i) {
    if (!volumes?.length || i < period - 1) return null;
    let s = 0;
    for (let j = 0; j < period; j++) s += volumes[i - j] || 0;
    return s / period;
  }
  function peakRsiBefore(rsi, endIdx, lookback = 5) {
    let peak = rsi[endIdx];
    if (peak == null) return 70;
    const start = Math.max(0, endIdx - lookback + 1);
    for (let j = start; j <= endIdx; j++) {
      const v = rsi[j];
      if (v != null && v > peak) peak = v;
    }
    return peak;
  }
  function calcTakeProfitPositionPct(sig, barIndex, bars, options = {}) {
    const overbought = options.overbought ?? 70;
    const rsi = sig?.rsi || [];
    const closes = bars?.closes || [];
    const ma20 = sig?.ma20 || [];
    const i = barIndex;
    if (i < 1 || rsi[i] == null || rsi[i - 1] == null) return 0.3;
    const peak = peakRsiBefore(rsi, i - 1, 5);
    const drop = rsi[i - 1] - rsi[i];
    let pct = 0.22 + Math.min(0.33, Math.max(0, (peak - overbought) / 25) * 0.33);
    pct += Math.min(0.12, Math.max(0, drop - 2) / 10 * 0.12);
    const c = closes[i];
    const m = ma20[i];
    if (c != null && m != null && m > 0 && c < m) pct += 0.08;
    return roundPct01(clamp(pct, 0.2, 0.65));
  }
  function calcReducePositionPct(sig, barIndex, bars, options = {}) {
    const rsi = sig?.rsi || [];
    const closes = bars?.closes || [];
    const volumes = bars?.volumes || [];
    const ma20 = sig?.ma20 || [];
    const volPeriod = options.volPeriod ?? 5;
    const i = barIndex;
    if (i < 0 || !closes[i] || !ma20[i] || ma20[i] <= 0) return 0.4;
    const breakPct = Math.max(0, (ma20[i] - closes[i]) / ma20[i]);
    let pct = 0.32 + Math.min(0.38, breakPct / 0.07 * 0.38);
    const vma = volMa(volumes, volPeriod, i);
    if (vma != null && vma > 0 && volumes[i]) {
      const vm = volumes[i] / vma;
      if (vm >= 2) pct += 0.12;
      else if (vm >= 1.4) pct += 0.07;
    }
    if (i >= 5 && ma20[i] != null && ma20[i - 5] != null && ma20[i] < ma20[i - 5]) {
      pct += 0.08;
    }
    const r = rsi[i];
    if (r != null) {
      if (r < 35) pct += 0.1;
      else if (r < 45) pct += 0.05;
    }
    return roundPct01(clamp(pct, 0.3, 0.85));
  }
  function calcSellPositionPct(tag, sig, barIndex, bars, options = {}) {
    if (tag === "\u6B62") {
      return calcTakeProfitPositionPct(sig, barIndex, bars, options);
    }
    if (tag === "\u51CF") {
      return calcReducePositionPct(sig, barIndex, bars, options);
    }
    return null;
  }
  function formatSellPositionPct(pct) {
    if (pct == null || !Number.isFinite(pct)) return "";
    return `${Math.round(pct * 100)}%`;
  }
  function sellPositionHint(tag, pct) {
    if (pct == null || !Number.isFinite(pct)) return "";
    const s = formatSellPositionPct(pct);
    if (tag === "\u6B62") return `\u5EFA\u8BAE\u6B62\u76C8 ${s}`;
    if (tag === "\u51CF") return `\u5EFA\u8BAE\u51CF\u4ED3 ${s}`;
    return "";
  }

  // src/utils/buyPriceRange.js
  var BUY_ENTRY_TAGS = /* @__PURE__ */ new Set(["\u5F3A", "\u8D8B", "\u8F6C", "\u7A81", "\u4E70", "\u5F39"]);
  var MAX_RANGE_PCT = 0.018;
  var DEFER_RANGE_PCT = 0.022;
  var TOMORROW_HEADROOM_PCT = 8e-3;
  function formatPriceTick(n) {
    if (!Number.isFinite(n)) return "\u2014";
    const abs = Math.abs(n);
    if (abs >= 100) return n.toFixed(2);
    if (abs >= 1) return n.toFixed(2);
    return n.toFixed(3);
  }
  function tightenRangeFromHigh(lo, hi, maxPct = MAX_RANGE_PCT) {
    if (!Number.isFinite(lo) || !Number.isFinite(hi)) return null;
    lo = Math.min(lo, hi);
    if (lo <= 0 || hi <= 0 || lo >= hi) return null;
    const maxSpan = hi * maxPct;
    if (hi - lo > maxSpan) lo = hi - maxSpan;
    if (lo >= hi) return null;
    return { low: lo, high: hi };
  }
  function smaAt(closes, idx, period = 20) {
    if (!closes?.length || idx < period - 1) return null;
    let s = 0;
    for (let j = 0; j < period; j++) s += closes[idx - j];
    return s / period;
  }
  function tomorrowRangeHigh(instantPrice) {
    return instantPrice * (1 + TOMORROW_HEADROOM_PCT);
  }
  function resolveInstantBarIndex(summary) {
    const tag = summary?.tag;
    const confirmBar = summary.recentSignalConfirmBar;
    const sigBar = summary.recentSignalBar;
    if ((tag === "\u5F3A" || tag === "\u7A81") && confirmBar != null && confirmBar >= 0) {
      return confirmBar;
    }
    return sigBar;
  }
  function priceStatus(nowClose, instantPrice, rangeHigh) {
    const hi = rangeHigh ?? instantPrice;
    if (nowClose == null || instantPrice == null || !Number.isFinite(instantPrice) || instantPrice <= 0) {
      return "unknown";
    }
    if (nowClose >= instantPrice * 0.992 && nowClose <= hi * 1.002) return "inZone";
    if (nowClose > hi * 1.005) return "above";
    if (nowClose < instantPrice * 0.985) return "below";
    return "near";
  }
  function rangeText(range) {
    if (!range) return null;
    return `${formatPriceTick(range.low)}~${formatPriceTick(range.high)}`;
  }
  function calcDeferRange(summary, bars, instantBar, instantPrice, sigLow, last, rangeHigh) {
    const lows = bars?.lows ?? [];
    const daysAgo = summary.recentSignalDaysAgo ?? Math.max(0, last - instantBar);
    const ma20 = smaAt(bars?.closes ?? [], last, 20);
    const hi = rangeHigh ?? instantPrice;
    let lo = sigLow;
    for (let i = instantBar; i <= last; i++) {
      const l = lows[i];
      if (Number.isFinite(l)) lo = Math.min(lo, l);
    }
    if (ma20 != null && ma20 > 0) lo = Math.min(lo, ma20);
    const maxPct = daysAgo === 0 ? DEFER_RANGE_PCT : MAX_RANGE_PCT;
    return tightenRangeFromHigh(lo, hi, maxPct);
  }
  function calcBuyPriceRange(summary, bars, _options = {}) {
    const tag = summary?.tag;
    if (!BUY_ENTRY_TAGS.has(tag)) return null;
    const closes = bars?.closes ?? [];
    const lows = bars?.lows ?? [];
    const opens = bars?.opens ?? [];
    const last = summary.signalLastIndex ?? closes.length - 1;
    if (last < 0 || closes[last] == null) return null;
    const instantBar = resolveInstantBarIndex(summary);
    if (instantBar == null || instantBar < 0 || instantBar > last || closes[instantBar] == null) {
      return null;
    }
    const instantPrice = closes[instantBar];
    const sigLow = lows[instantBar] ?? Math.min(opens[instantBar] ?? instantPrice, instantPrice);
    const nowClose = closes[last];
    const daysAgo = summary.recentSignalDaysAgo ?? Math.max(0, last - instantBar);
    const rangeHigh = daysAgo === 0 ? tomorrowRangeHigh(instantPrice) : instantPrice;
    const idealPullback = tightenRangeFromHigh(sigLow, instantPrice);
    const displayPullback = tightenRangeFromHigh(sigLow, rangeHigh);
    const deferPullback = calcDeferRange(summary, bars, instantBar, instantPrice, sigLow, last, rangeHigh);
    const status = priceStatus(nowClose, instantPrice, rangeHigh);
    const idealText = rangeText(idealPullback);
    const displayTextRaw = rangeText(displayPullback);
    const deferText = rangeText(deferPullback);
    let displayText = daysAgo === 0 ? displayTextRaw : idealText;
    let deferMode = "same";
    let statusHint = "";
    if (status === "above") {
      deferMode = "wait";
      displayText = deferText || displayTextRaw || idealText;
      statusHint = daysAgo === 0 ? `\u4ECA\u65E5\u53EF\u4E0D\u8FFD\uFF0C\u660E\u65E5\u7B49\u56DE\u8E29 ${displayText || "\u533A\u95F4\u9644\u8FD1"}` : `\u73B0\u4EF7\u504F\u9AD8\uFF0C\u7B49\u56DE\u8E29 ${displayText || "\u533A\u95F4\u9644\u8FD1"}`;
    } else if (status === "below") {
      deferMode = daysAgo === 0 ? "todayOrTomorrow" : "defer";
      displayText = deferText || displayTextRaw || idealText;
      statusHint = `\u73B0\u4EF7${formatPriceTick(nowClose)}\u4F4E\u4E8E\u51FA\u4FE1\u53F7\u4EF7\uFF0C\u533A\u95F4\u4ECD\u6709\u6548`;
    } else if (status === "inZone" || status === "near") {
      deferMode = daysAgo === 0 ? "todayOrTomorrow" : "same";
      displayText = displayTextRaw || idealText || deferText;
      statusHint = daysAgo === 0 ? `\u660E\u65E5\u4ECD\u53EF\u4E70\uFF0C\u4EF7\u683C\u843D\u5165 ${displayText || "\u53C2\u8003\u533A\u95F4"} \u5373\u53EF` : "\u73B0\u4EF7\u63A5\u8FD1\u51FA\u4FE1\u53F7\u4EF7";
    } else if (daysAgo > 0 && deferText) {
      deferMode = "defer";
      displayText = deferText;
      statusHint = `${daysAgo}\u65E5\u524D\u51FA\u4FE1\u53F7\uFF0C\u4ECD\u53EF\u7B49\u56DE\u8E29\u8BE5\u533A\u95F4`;
    }
    const tagNote = tag === "\u5F3A" || tag === "\u7A81" ? "\u51FA\u4FE1\u53F7\u4EF7=\u786E\u8BA4\u5B8C\u6210\u5F53\u65E5\u6536\u76D8\u4EF7" : "\u51FA\u4FE1\u53F7\u4EF7=\u6807\u4FE1\u53F7\u5F53\u65E5\u6536\u76D8\u4EF7";
    const instantText = status === "above" ? `${formatPriceTick(instantPrice)} \u2191` : status === "below" ? `${formatPriceTick(instantPrice)} \u2193` : formatPriceTick(instantPrice);
    const ma20 = smaAt(closes, last, 20);
    const tomorrowHighNote = daysAgo === 0 && rangeHigh > instantPrice ? `\u660E\u65E5\u4E0A\u9650\u2248${formatPriceTick(rangeHigh)}\uFF08\u51FA\u4FE1\u53F7\u4EF7+0.8%\uFF09` : "";
    return {
      tag,
      daysAgo,
      instantPrice,
      rangeHigh,
      instantBar,
      signalClose: instantPrice,
      signalLow: sigLow,
      low: displayPullback?.low ?? deferPullback?.low ?? sigLow,
      high: displayPullback?.high ?? rangeHigh,
      text: displayText,
      signalRefText: idealText,
      deferText,
      instantText,
      mode: status,
      deferMode,
      extended: status === "above",
      note: [
        tagNote,
        idealText ? `\u7406\u60F3\u8D34\u8FD1 ${idealText}` : "",
        tomorrowHighNote,
        deferText && deferText !== displayTextRaw && deferText !== idealText ? `\u6DF1\u56DE\u8E29 ${deferText}` : "",
        ma20 != null ? `MA20\u2248${formatPriceTick(ma20)}` : "",
        statusHint
      ].filter(Boolean).join(" \xB7 ")
    };
  }

  // src/utils/icePointSignals.js
  var SELL_TAG_TAKE_PROFIT = "\u6B62";
  var SELL_TAG_REDUCE = "\u51CF";
  var SIGNAL_TAG_PRIORITY = ["\u51CF", "\u6B62", "\u51B2", "\u52A0", "\u5F3A", "\u8D8B", "\u8F6C", "\u7A81", "\u5F39", "\u4E70", "\u51B0"];
  var SIGNAL_SORT_RANK_NONE = 10;
  function signalSortRank(tag) {
    if (!tag) return SIGNAL_SORT_RANK_NONE;
    const i = SIGNAL_TAG_PRIORITY.indexOf(tag);
    return i >= 0 ? i : SIGNAL_SORT_RANK_NONE;
  }
  function isSellSignalTag(tag) {
    return tag === SELL_TAG_TAKE_PROFIT || tag === SELL_TAG_REDUCE;
  }
  function calcRSI(closes, period = 14) {
    const out = new Array(closes.length).fill(null);
    for (let i = period; i < closes.length; i++) {
      let gain = 0;
      let loss = 0;
      for (let j = 0; j < period; j++) {
        const ch = closes[i - j] - closes[i - j - 1];
        if (ch >= 0) gain += ch;
        else loss -= ch;
      }
      const ag = gain / period;
      const al = loss / period;
      out[i] = al === 0 ? 100 : 100 - 100 / (1 + ag / al);
    }
    return out;
  }
  function sma(closes, period) {
    const out = new Array(closes.length).fill(null);
    for (let i = period - 1; i < closes.length; i++) {
      let s = 0;
      for (let j = 0; j < period; j++) s += closes[i - j];
      out[i] = s / period;
    }
    return out;
  }
  function volMa2(volumes, period, i) {
    if (!volumes?.length || i < period - 1) return null;
    let s = 0;
    for (let j = 0; j < period; j++) s += volumes[i - j] || 0;
    return s / period;
  }
  function normalizeDayKey(dayStr) {
    const s = String(dayStr || "").trim().replace(/\//g, "-");
    const m = s.match(/^(\d{4})-(\d{2})-(\d{2})/);
    if (m) return `${m[1]}-${m[2]}-${m[3]}`;
    const c = s.match(/^(\d{4})(\d{2})(\d{2})/);
    if (c) return `${c[1]}-${c[2]}-${c[3]}`;
    return s.slice(0, 10);
  }
  function buildIndexMa20ByDay(indexCloseByDay, maPeriod = 20) {
    const out = /* @__PURE__ */ new Map();
    if (!indexCloseByDay?.size) return out;
    const days = [...indexCloseByDay.keys()].sort();
    const closes = days.map((d) => indexCloseByDay.get(d));
    for (let i = 0; i < days.length; i++) {
      let ma = null;
      if (i >= maPeriod - 1) {
        let s = 0;
        for (let j = 0; j < maPeriod; j++) s += closes[i - j];
        ma = s / maPeriod;
      }
      out.set(days[i], { close: closes[i], ma20: ma });
    }
    return out;
  }
  function lastIceBefore(iceEnter, i, minGap) {
    let found = null;
    for (const idx of iceEnter) {
      if (idx < i && i - idx >= minGap) found = idx;
    }
    return found;
  }
  function indexBullishOnDay(dayKey, indexMa20ByDay) {
    if (!indexMa20ByDay?.size || !dayKey) return true;
    const row = indexMa20ByDay.get(dayKey);
    if (!row || row.ma20 == null) return true;
    return row.close > row.ma20;
  }
  function computeTradeSignals(closes, options = {}) {
    const {
      rsiPeriod = 14,
      iceThreshold = 30,
      overbought = 70,
      lookback = 5,
      maPeriod = 20,
      reduceConfirmDays = 1,
      takeProfitMinOverboughtDays = 3
    } = options;
    const len = closes.length;
    const rsi = calcRSI(closes, rsiPeriod);
    const ma20 = sma(closes, maPeriod);
    const iceEnter = [];
    const buy = [];
    const sellRsi = [];
    const sellMa20 = [];
    for (let i = 1; i < len; i++) {
      const r = rsi[i];
      const prev = rsi[i - 1];
      if (r == null || prev == null) continue;
      if (prev >= iceThreshold && r < iceThreshold) {
        iceEnter.push(i);
      }
      if (prev <= iceThreshold && r > iceThreshold) {
        let hadFormalIce = false;
        for (const idx of iceEnter) {
          if (idx < i && i - idx <= lookback) {
            hadFormalIce = true;
            break;
          }
        }
        if (hadFormalIce) buy.push(i);
      }
      if (prev >= overbought && r < overbought) {
        const minDays = Math.max(1, Math.floor(Number(takeProfitMinOverboughtDays) || 1));
        let streak = 0;
        for (let k = i - 1; k >= 0 && streak < minDays; k--) {
          const rk = rsi[k];
          if (rk == null || rk < overbought) break;
          streak++;
        }
        if (streak >= minDays) sellRsi.push(i);
      }
    }
    const reduceConfirm = Math.max(1, Math.floor(Number(reduceConfirmDays) || 1));
    for (let i = reduceConfirm - 1; i < len; i++) {
      let allBelow = true;
      for (let j = 0; j < reduceConfirm; j++) {
        const idx = i - j;
        if (ma20[idx] == null || closes[idx] == null || closes[idx] >= ma20[idx]) {
          allBelow = false;
          break;
        }
      }
      if (!allBelow) continue;
      const before = i - reduceConfirm;
      if (before >= 0 && ma20[before] != null && closes[before] != null && closes[before] < ma20[before]) {
        continue;
      }
      sellMa20.push(i);
    }
    const last = len - 1;
    let latestStatus = { text: "\u2014", type: "default", rsi: null, ma20: null };
    if (last >= 0 && rsi[last] != null) {
      const rl = rsi[last];
      latestStatus.rsi = rl;
      if (ma20[last] != null) latestStatus.ma20 = ma20[last];
      if (rl < iceThreshold) {
        latestStatus = {
          ...latestStatus,
          text: `\u51B0\u70B9\u533A\u5185 \xB7 RSI ${rl.toFixed(1)}`,
          type: "info"
        };
      } else if (rl > overbought) {
        latestStatus = {
          ...latestStatus,
          text: `\u8D85\u4E70\u533A \xB7 RSI ${rl.toFixed(1)}`,
          type: "warning"
        };
      } else {
        let daysSinceBuy = null;
        for (let i = last; i >= 0; i--) {
          if (buy.includes(i)) {
            daysSinceBuy = last - i;
            break;
          }
        }
        if (daysSinceBuy != null && daysSinceBuy <= 15) {
          latestStatus = {
            ...latestStatus,
            text: `\u51FA\u51B0\u70B9 ${daysSinceBuy} \u4E2A\u4EA4\u6613\u65E5\u524D \xB7 RSI ${rl.toFixed(1)}`,
            type: "success"
          };
        } else {
          latestStatus = {
            ...latestStatus,
            text: `\u5E38\u6001 \xB7 RSI ${rl.toFixed(1)}`,
            type: "default"
          };
        }
      }
    }
    return { rsi, ma20, iceEnter, buy, sellRsi, sellMa20, latestStatus };
  }
  function computeBreakoutSignals(bars, options = {}) {
    const {
      closes = [],
      opens = [],
      highs = [],
      lows = [],
      volumes = [],
      boxPeriod = 20,
      maxRangePct = 0.2,
      volMult = 1.25,
      volPeriod = 5,
      minGap = 15,
      breakBuffer = 5e-3,
      /** 突破后需连续站稳的交易日数（不含突破日本身） */
      confirmDays = 3,
      /** 突破日 K 线上影线占振幅上限；null 关闭 */
      maxUpperWickRatio = 0.55
    } = { ...bars, ...options };
    const len = closes.length;
    const confirm = Math.max(0, Math.floor(Number(confirmDays) || 0));
    if (len < boxPeriod + confirm + 1) return [];
    const ma5 = sma(closes, 5);
    const ma10 = sma(closes, 10);
    const ma20 = sma(closes, 20);
    const breakout = [];
    function boxRange(breakIdx) {
      const lookStart = breakIdx - boxPeriod;
      const lookEnd = breakIdx - 1;
      if (lookStart < 0) return null;
      let boxHigh = -Infinity;
      let boxLow = Infinity;
      for (let j = lookStart; j <= lookEnd; j++) {
        const h = highs[j] ?? Math.max(opens[j] ?? closes[j], closes[j]);
        const l = lows[j] ?? Math.min(opens[j] ?? closes[j], closes[j]);
        boxHigh = Math.max(boxHigh, h);
        boxLow = Math.min(boxLow, l);
      }
      const mid = (boxHigh + boxLow) / 2;
      if (mid <= 0 || boxHigh <= boxLow) return null;
      if ((boxHigh - boxLow) / mid > maxRangePct) return null;
      return { boxHigh, boxLow };
    }
    function passesBreakoutDay(breakIdx, boxHigh) {
      const o = opens[breakIdx];
      const c = closes[breakIdx];
      if (o == null || c == null || c <= o) return false;
      if (c <= boxHigh * (1 + breakBuffer)) return false;
      const vma = volMa2(volumes, volPeriod, breakIdx);
      if (vma != null && vma > 0 && (volumes[breakIdx] || 0) < vma * volMult) return false;
      if (ma5[breakIdx] == null || ma10[breakIdx] == null) return false;
      if (ma5[breakIdx] < ma10[breakIdx]) return false;
      if (ma20[breakIdx] != null && ma10[breakIdx] < ma20[breakIdx] * 0.995) return false;
      if (maxUpperWickRatio != null) {
        const h = highs[breakIdx] ?? Math.max(o, c);
        const l = lows[breakIdx] ?? Math.min(o, c);
        const range = h - l;
        if (range > 0) {
          const upperWick = h - Math.max(o, c);
          if (upperWick / range > maxUpperWickRatio) return false;
        }
      }
      return true;
    }
    function holdsAboveBox(breakIdx, boxHigh, throughIdx) {
      const holdFloor = boxHigh;
      for (let j = breakIdx + 1; j <= throughIdx; j++) {
        const c = closes[j];
        if (c == null || c < holdFloor) return false;
      }
      return true;
    }
    for (let breakIdx = boxPeriod; breakIdx < len; breakIdx++) {
      const box = boxRange(breakIdx);
      if (!box || !passesBreakoutDay(breakIdx, box.boxHigh)) continue;
      const confirmEnd = breakIdx + confirm;
      if (confirmEnd >= len) continue;
      if (!holdsAboveBox(breakIdx, box.boxHigh, confirmEnd)) continue;
      let dup = false;
      for (const b of breakout) {
        if (breakIdx - b < minGap) {
          dup = true;
          break;
        }
      }
      if (!dup) breakout.push(breakIdx);
    }
    return breakout;
  }
  function computeReboundAfterSell(bars, baseSig, skipIndices, options = {}) {
    const { closes = [], opens = [], highs = [], lows = [] } = bars || {};
    const {
      sellLookback = 6,
      minReboundPct = 0.05,
      minBodyPct = 0.015,
      maxRsi = 60,
      minGap = 6,
      buyIndices = [],
      buyAdjacentDays = 2,
      minDaysAfterSell = 1,
      maxUpperWickRatio = 0.5,
      confirmDays = 2
    } = options;
    const confirm = Math.max(1, Math.floor(Number(confirmDays) || 2));
    const len = closes.length;
    const ma20 = baseSig?.ma20;
    const ma5 = sma(closes, 5);
    const ma10 = sma(closes, 10);
    const rsi = baseSig?.rsi;
    const sellSet = /* @__PURE__ */ new Set([...baseSig?.sellRsi || [], ...baseSig?.sellMa20 || []]);
    const skip = skipIndices instanceof Set ? skipIndices : new Set(skipIndices || []);
    const reboundBuy = [];
    function lastSellBefore(i) {
      for (let j = i - 1; j >= Math.max(0, i - sellLookback); j--) {
        if (sellSet.has(j)) return j;
      }
      return null;
    }
    function nearBuySignal(i) {
      for (const b of buyIndices) {
        if (Math.abs(i - b) <= buyAdjacentDays) return true;
      }
      return false;
    }
    function holdsAboveMa20(fromIdx, throughIdx) {
      for (let j = fromIdx; j <= throughIdx; j++) {
        const c = closes[j];
        const m = ma20?.[j];
        if (c == null || m == null || c < m) return false;
      }
      return true;
    }
    function wasBelowMa20Since(sellIdx, beforeIdx) {
      for (let j = sellIdx; j < beforeIdx; j++) {
        const c = closes[j];
        const m = ma20?.[j];
        if (c != null && m != null && c < m) return true;
      }
      return false;
    }
    function passesSignalDayFilters(i) {
      const o = opens[i];
      const c = closes[i];
      const h = highs[i];
      const l = lows[i];
      if (o == null || c == null || c <= o) return false;
      if (o <= 0 || (c - o) / o < minBodyPct) return false;
      const r = rsi?.[i];
      if (r == null || r > maxRsi) return false;
      const m5v = ma5[i];
      const m10v = ma10[i];
      if (m5v == null || c <= m5v) return false;
      if (m10v == null || m5v < m10v) return false;
      const m20v = ma20?.[i];
      const m20prev = ma20?.[i - 1];
      if (m20v == null || m20prev == null || m20v < m20prev) return false;
      if (maxUpperWickRatio != null && h != null && l != null && h > l) {
        const upperWick = (h - c) / (h - l);
        if (upperWick > maxUpperWickRatio) return false;
      }
      return true;
    }
    for (let i = confirm - 1; i < len; i++) {
      if (skip.has(i)) continue;
      if (nearBuySignal(i)) continue;
      if (!passesSignalDayFilters(i)) continue;
      const standStart = i - confirm + 1;
      if (standStart < 1) continue;
      if (!holdsAboveMa20(standStart, i)) continue;
      const sellIdx = lastSellBefore(i);
      if (sellIdx == null || i - sellIdx < minDaysAfterSell) continue;
      if (standStart <= sellIdx) continue;
      let lowSince = Infinity;
      for (let j = sellIdx; j <= i; j++) {
        const lj = lows[j] ?? closes[j];
        if (lj != null && lj < lowSince) lowSince = lj;
      }
      const c = closes[i];
      const reboundPctOk = lowSince > 0 && lowSince < Infinity && c != null && (c - lowSince) / lowSince >= minReboundPct;
      if (!wasBelowMa20Since(sellIdx, standStart) && !reboundPctOk) continue;
      let dup = false;
      for (const b of reboundBuy) {
        if (i - b < minGap) {
          dup = true;
          break;
        }
      }
      if (!dup) reboundBuy.push(i);
    }
    return reboundBuy;
  }
  function computeReversalSignals(bars, baseSig, skipIndices, options = {}) {
    if (options.reversalEnabled === false) return [];
    const {
      closes = [],
      opens = []
    } = bars || {};
    const len = closes.length;
    const crossPeriod = Math.max(5, Math.floor(Number(options.reversalCrossMaPeriod) || 60));
    if (len < crossPeriod + 1) return [];
    const maCross = sma(closes, crossPeriod);
    const ma5 = sma(closes, 5);
    const ma10 = sma(closes, 10);
    const ma20 = baseSig?.ma20 ?? sma(closes, 20);
    const rsi = baseSig?.rsi;
    const skip = skipIndices instanceof Set ? skipIndices : new Set(skipIndices || []);
    const minBodyPct = options.reversalMinBodyPct ?? 0.01;
    const rsiMin = options.reversalRsiMin ?? 45;
    const rsiMax = options.reversalRsiMax ?? 70;
    const requireMa20FlatOrUp = options.reversalRequireMa20FlatOrUp !== false;
    const ma20Lookback = Math.max(1, Math.floor(Number(options.reversalMa20LookbackDays) || 5));
    const requireMa5AboveMa10 = options.reversalRequireMa5AboveMa10 === true;
    const requireCloseAboveMa5 = options.reversalRequireCloseAboveMa5 !== false;
    const recentBelowDays = Math.max(1, Math.floor(Number(options.reversalRecentBelowDays) || 5));
    const recentBelowMinRatio = options.reversalRecentBelowMinRatio ?? 0.6;
    const minGap = Math.max(1, Math.floor(Number(options.reversalMinGap) || 5));
    const minBelowCount = Math.max(1, Math.ceil(recentBelowDays * recentBelowMinRatio));
    const reversalBuy = [];
    for (let i = crossPeriod; i < len; i++) {
      if (skip.has(i)) continue;
      const mc = maCross[i];
      const mcPrev = maCross[i - 1];
      const c = closes[i];
      const cPrev = closes[i - 1];
      if (mc == null || mcPrev == null || c == null || cPrev == null) continue;
      if (c <= mc || cPrev > mcPrev) continue;
      let belowCount = 0;
      for (let j = Math.max(0, i - recentBelowDays); j < i; j++) {
        if (closes[j] != null && maCross[j] != null && closes[j] <= maCross[j]) belowCount++;
      }
      if (belowCount < minBelowCount) continue;
      const o = opens[i];
      if (o == null || c <= o) continue;
      if (o <= 0 || (c - o) / o < minBodyPct) continue;
      if (requireCloseAboveMa5 && (ma5[i] == null || c <= ma5[i])) continue;
      if (requireMa5AboveMa10 && (ma5[i] == null || ma10[i] == null || ma5[i] <= ma10[i])) continue;
      const r = rsi?.[i];
      if (r == null || r < rsiMin || r > rsiMax) continue;
      if (requireMa20FlatOrUp) {
        const m20 = ma20[i];
        const m20Prev = i >= ma20Lookback ? ma20[i - ma20Lookback] : null;
        if (m20 == null || m20Prev == null || m20 < m20Prev) continue;
      }
      let dup = false;
      for (const t of reversalBuy) {
        if (i - t < minGap) {
          dup = true;
          break;
        }
      }
      if (!dup) reversalBuy.push(i);
    }
    return reversalBuy;
  }
  function passesTrendChopFilter(i, ctx = {}) {
    const { options = {}, ma5, ma10, closes, highs, lows, sellSet } = ctx;
    if (options.trendChopFilterEnabled === false) return true;
    const m5 = ma5?.[i];
    const m10 = ma10?.[i];
    const minSpread = options.trendMinMaSpreadPct ?? 0.01;
    if (m10 != null && m10 > 0 && m5 != null && (m5 - m10) / m10 < minSpread) {
      return false;
    }
    const sellLb = Math.max(0, Math.floor(Number(options.trendChopSellLookback) || 8));
    if (sellSet?.size && sellLb > 0) {
      for (let j = Math.max(0, i - sellLb); j < i; j++) {
        if (sellSet.has(j)) return false;
      }
    }
    const rangeDays = Math.max(2, Math.floor(Number(options.trendChopRangeDays) || 5));
    const maxRangePct = options.trendChopMaxRangePct ?? 0.08;
    if (maxRangePct > 0) {
      const start = i - rangeDays + 1;
      if (start >= 0) {
        let hi = -Infinity;
        let lo = Infinity;
        for (let j = start; j <= i; j++) {
          const h = highs?.[j] ?? closes?.[j];
          const l = lows?.[j] ?? closes?.[j];
          if (h != null) hi = Math.max(hi, h);
          if (l != null) lo = Math.min(lo, l);
        }
        const mid = (hi + lo) / 2;
        if (mid > 0 && hi > lo && (hi - lo) / mid < maxRangePct) return false;
      }
    }
    return true;
  }
  function collectEntryBarGroups(filteredBuy, strictBuy, trendBuy, reversalBuy, breakoutBuy, reboundBuy) {
    return {
      buy: filteredBuy || [],
      strong: strictBuy || [],
      trend: trendBuy || [],
      reversal: reversalBuy || [],
      breakout: breakoutBuy || [],
      rebound: reboundBuy || []
    };
  }
  function resolveSellProtectDays(options = {}) {
    const legacy = options.sellProtectAfterEntryDays;
    const fallback = legacy != null ? legacy : 0;
    return {
      buy: options.sellProtectAfterBuy ?? fallback,
      strong: options.sellProtectAfterStrong ?? fallback,
      trend: options.sellProtectAfterTrend ?? fallback,
      breakout: options.sellProtectAfterBreakout ?? fallback,
      rebound: options.sellProtectAfterRebound ?? fallback,
      reversal: options.sellProtectAfterReversal ?? fallback
    };
  }
  function buildEntryMetaList(groups, lows, closes, protectByType) {
    const out = [];
    for (const [type, indices] of Object.entries(groups)) {
      const protect = Math.max(0, Math.floor(Number(protectByType[type]) || 0));
      for (const e of indices) {
        out.push({
          index: e,
          type,
          protect,
          low: lows[e] ?? closes[e]
        });
      }
    }
    return out;
  }
  function nearestEntryBefore(barIndex, entries) {
    let nearest = null;
    for (const ent of entries) {
      if (ent.index < barIndex && (!nearest || ent.index > nearest.index)) nearest = ent;
    }
    return nearest;
  }
  function filterTakeProfitWithProfitGate(indices, entryMeta, closes, options) {
    if (options.sellProfitGateEnabled === false) return indices || [];
    const minGain = options.takeProfitMinGainPct ?? 0.05;
    const lookback = options.sellBreakEntryLowLookback ?? options.recentBuyDays ?? 15;
    const minPullback = options.takeProfitMinPullbackFromPeakPct ?? 0.03;
    const requirePullback = options.takeProfitRequirePullbackFromPeak !== false;
    return (indices || []).filter((i) => {
      const c = closes[i];
      if (c == null) return false;
      const ent = nearestEntryBefore(i, entryMeta);
      let entryPrice = null;
      let peakFrom = 0;
      if (ent && i - ent.index <= lookback) {
        entryPrice = ent.low ?? closes[ent.index];
        peakFrom = ent.index;
      } else {
        const start = Math.max(0, i - lookback);
        let refLow = Infinity;
        for (let j = start; j < i; j++) {
          if (closes[j] != null) refLow = Math.min(refLow, closes[j]);
        }
        entryPrice = Number.isFinite(refLow) ? refLow : null;
        peakFrom = start;
      }
      if (entryPrice == null || entryPrice <= 0) return false;
      if ((c - entryPrice) / entryPrice < minGain) return false;
      if (requirePullback) {
        let peak = -Infinity;
        for (let j = peakFrom; j <= i; j++) {
          if (closes[j] != null) peak = Math.max(peak, closes[j]);
        }
        if (peak > 0 && (peak - c) / peak < minPullback) return false;
      }
      return true;
    });
  }
  function augmentEarlyReduceSignals(sellMa20, bars, ma20, options = {}) {
    if (options.reduceEarlyEnabled === false) return sellMa20 || [];
    const { closes = [], opens = [], lows = [] } = bars || {};
    const len = closes.length;
    if (len < 3) return sellMa20 || [];
    const set = new Set(sellMa20 || []);
    const ma5 = sma(closes, 5);
    const ma5Days = Math.max(1, Math.floor(Number(options.reduceEarlyMa5Days) || 3));
    const minDrop = options.reduceEarlyMinDropPct ?? 0.045;
    const breakLowDays = Math.max(2, Math.floor(Number(options.reduceEarlyBreakLowDays) || 5));
    const uptrendLookback = Math.max(3, Math.floor(Number(options.reduceEarlyUptrendLookback) || 8));
    const requireUptrend = options.reduceEarlyRequireUptrend !== false;
    function hadRecentAboveMa20(i) {
      if (!requireUptrend) return true;
      for (let j = Math.max(0, i - uptrendLookback); j < i; j++) {
        if (closes[j] != null && ma20[j] != null && closes[j] >= ma20[j]) return true;
      }
      return false;
    }
    for (let i = 1; i < len; i++) {
      if (set.has(i)) continue;
      if (!hadRecentAboveMa20(i)) continue;
      const c = closes[i];
      const cPrev = closes[i - 1];
      const mPrev = ma20[i - 1];
      if (c == null || cPrev == null) continue;
      if (i >= ma5Days) {
        let belowMa5 = true;
        for (let j = 0; j < ma5Days; j++) {
          const idx = i - j;
          if (ma5[idx] == null || closes[idx] == null || closes[idx] >= ma5[idx]) {
            belowMa5 = false;
            break;
          }
        }
        if (belowMa5) {
          const before = i - ma5Days;
          const wasAboveMa5 = before >= 0 && ma5[before] != null && closes[before] != null && closes[before] >= ma5[before];
          const stillNearMa20 = ma20[i] != null && c >= ma20[i] * 0.985;
          if (wasAboveMa5 && stillNearMa20) set.add(i);
        }
      }
      if (mPrev != null && cPrev >= mPrev && cPrev > 0) {
        const drop = (cPrev - c) / cPrev;
        const o = opens[i];
        if (drop >= minDrop && o != null && c < o) set.add(i);
      }
      if (mPrev != null && cPrev >= mPrev && i >= breakLowDays) {
        let priLo = Infinity;
        for (let j = i - breakLowDays; j < i; j++) {
          const lj = lows[j] ?? closes[j];
          if (lj != null) priLo = Math.min(priLo, lj);
        }
        if (Number.isFinite(priLo) && c < priLo) set.add(i);
      }
    }
    return [...set].sort((a, b) => a - b);
  }
  function filterSellSignalsAfterEntry(sellIndices, entries, closes, options = {}) {
    if (!sellIndices?.length) return [];
    const allowBreakLow = options.sellBreakEntryLowEnablesReduce !== false;
    const isReduce = options.sellKind === "reduce";
    return sellIndices.filter((i) => {
      const ent = nearestEntryBefore(i, entries);
      if (!ent || ent.protect < 1) return true;
      const gap = i - ent.index;
      if (gap > ent.protect) return true;
      if (isReduce && allowBreakLow) {
        const c = closes[i];
        const floor = ent.low;
        if (c != null && floor != null && c < floor) return true;
      }
      return false;
    });
  }
  function dedupeSignalIndices(indices, minGap) {
    const gap = Math.max(0, Math.floor(Number(minGap) || 0));
    const sorted = [...indices || []].sort((a, b) => a - b);
    if (gap < 1 || sorted.length <= 1) return sorted;
    const out = [sorted[0]];
    for (let k = 1; k < sorted.length; k++) {
      if (sorted[k] - out[out.length - 1] >= gap) out.push(sorted[k]);
    }
    return out;
  }
  function augmentReduceByEntryLowBreak(sellMa20, closes, entries, lookback = 15) {
    const set = new Set(sellMa20 || []);
    const len = closes.length;
    for (let i = 1; i < len; i++) {
      let nearest = null;
      for (const ent of entries) {
        if (ent.index < i && i - ent.index <= lookback) {
          if (!nearest || ent.index > nearest.index) nearest = ent;
        }
      }
      if (!nearest) continue;
      const c = closes[i];
      const prev = closes[i - 1];
      const floor = nearest.low;
      if (c == null || prev == null || floor == null) continue;
      if (c < floor && prev >= floor) set.add(i);
    }
    return [...set].sort((a, b) => a - b);
  }
  function computeFullSignals(bars, options = {}) {
    const {
      closes = [],
      opens = [],
      highs = [],
      lows = [],
      volumes = [],
      dayKeys = [],
      indexMa20ByDay = null,
      lookback = 5,
      iceMinGap = 4,
      requireIndexBull = true,
      volPeriod = 5,
      iceThreshold = 30,
      /** 强买：成交量 ≥ 近均量 × 该倍数 */
      strictVolMult = 1.25,
      /** 强买：出「强」前需连续站稳的交易日数（不含买点日） */
      strongConfirmDays = 2,
      requireStockAboveMa20 = true,
      requireMa20Rising = true
    } = bars || {};
    const base = computeTradeSignals(closes, { lookback, iceThreshold, ...options });
    const len = closes.length;
    const ma5 = sma(closes, 5);
    const ma10 = sma(closes, 10);
    const strictBuy = [];
    const trendBuy = [];
    const strictSet = /* @__PURE__ */ new Set();
    const buyMinBodyPct = options.buyMinBodyPct ?? 0.01;
    function passesBuyDisplayFilter(i) {
      if (!opens?.length) return true;
      const o = opens[i];
      const c = closes[i];
      if (o == null || c == null || c <= o) return false;
      if (o <= 0 || (c - o) / o < buyMinBodyPct) return false;
      const m5v = ma5[i];
      if (m5v == null || c <= m5v) return false;
      const m20v = base.ma20[i];
      const m20prev = i > 0 ? base.ma20[i - 1] : null;
      if (m20v != null && c < m20v) {
        if (m20prev == null || m20v < m20prev) return false;
        const m10v = ma10[i];
        if (m10v == null || m5v < m10v) return false;
      }
      return true;
    }
    const filteredBuy = base.buy.filter(passesBuyDisplayFilter);
    const strongConfirm = Math.max(0, Math.floor(Number(strongConfirmDays) || 0));
    function holdsAfterBuy(buyIdx, throughIdx) {
      const floor = lows[buyIdx] ?? closes[buyIdx];
      if (floor == null) return false;
      for (let j = buyIdx + 1; j <= throughIdx; j++) {
        const c = closes[j];
        if (c == null || c < floor) return false;
      }
      return true;
    }
    function passesStrictBuyDay(buyIdx) {
      if (!filteredBuy.includes(buyIdx)) return false;
      if (lastIceBefore(base.iceEnter, buyIdx, iceMinGap) == null) return false;
      const o = opens[buyIdx];
      const c = closes[buyIdx];
      if (o == null || c == null || c <= o) return false;
      if (ma5[buyIdx] == null || c <= ma5[buyIdx]) return false;
      const vma = volMa2(volumes, volPeriod, buyIdx);
      if (vma != null && vma > 0 && (volumes[buyIdx] || 0) < vma * strictVolMult) return false;
      if (requireIndexBull && !indexBullishOnDay(dayKeys[buyIdx], indexMa20ByDay)) return false;
      if (requireStockAboveMa20) {
        const m20 = base.ma20[buyIdx];
        if (m20 == null || c <= m20) return false;
      }
      if (requireMa20Rising) {
        const m20 = base.ma20[buyIdx];
        const m20Prev = buyIdx > 0 ? base.ma20[buyIdx - 1] : null;
        if (m20 == null || m20Prev == null || m20 < m20Prev) return false;
      }
      return true;
    }
    for (const buyIdx of filteredBuy) {
      const confirmEnd = buyIdx + strongConfirm;
      if (confirmEnd >= len) continue;
      if (!passesStrictBuyDay(buyIdx)) continue;
      if (strongConfirm > 0 && !holdsAfterBuy(buyIdx, confirmEnd)) continue;
      let dup = false;
      for (const b of strictBuy) {
        if (buyIdx - b < iceMinGap) {
          dup = true;
          break;
        }
      }
      if (!dup) {
        strictBuy.push(buyIdx);
        strictSet.add(buyIdx);
      }
    }
    const trendSellSet = /* @__PURE__ */ new Set([...base.sellRsi || [], ...base.sellMa20 || []]);
    const trendChopCtx = { options, ma5, ma10, closes, highs, lows, sellSet: trendSellSet };
    for (let i = 20; i < len; i++) {
      if (filteredBuy.includes(i) || strictSet.has(i)) continue;
      const trendRsiMin = options.trendRsiMin ?? 45;
      const trendRsiMax = options.trendRsiMax ?? 68;
      const trendTouchMaPct = options.trendTouchMaPct ?? 0.015;
      const trendMaxCloseAboveMa5Pct = options.trendMaxCloseAboveMa5Pct ?? 0.03;
      const trendMinUpBars = options.trendMinUpBars ?? 3;
      const trendMinGap = options.trendMinGap ?? 5;
      const r = base.rsi[i];
      if (r == null || r < trendRsiMin || r > trendRsiMax) continue;
      if (ma5[i] == null || ma10[i] == null || base.ma20[i] == null) continue;
      if (closes[i - 1] <= base.ma20[i - 1] || ma5[i - 1] <= ma10[i - 1]) continue;
      let upBars = 0;
      for (let j = Math.max(0, i - 4); j < i; j++) {
        if (closes[j] > base.ma20[j]) upBars++;
      }
      if (upBars < trendMinUpBars) continue;
      const low = lows[i] ?? closes[i];
      const touchMa5 = low <= ma5[i] * (1 + trendTouchMaPct);
      const touchMa10 = low <= ma10[i] * (1 + trendTouchMaPct);
      if (!touchMa5 && !touchMa10) continue;
      const o = opens[i];
      const c = closes[i];
      if (o == null || c == null || c <= o || c <= ma5[i]) continue;
      if (trendMaxCloseAboveMa5Pct > 0 && ma5[i] > 0) {
        if ((c - ma5[i]) / ma5[i] > trendMaxCloseAboveMa5Pct) continue;
      }
      if (!passesTrendChopFilter(i, trendChopCtx)) continue;
      const trendRequireNextDayYang = options.trendRequireNextDayYang === true;
      let markIdx = i;
      if (trendRequireNextDayYang) {
        const next = i + 1;
        if (next >= len) continue;
        const nOpen = opens[next];
        const nClose = closes[next];
        if (nOpen == null || nClose == null || nClose <= nOpen) continue;
        markIdx = next;
      }
      let dup = false;
      for (const t of trendBuy) {
        if (markIdx - t < trendMinGap) {
          dup = true;
          break;
        }
      }
      if (!dup) trendBuy.push(markIdx);
    }
    const breakoutBuy = computeBreakoutSignals(
      { closes, opens, highs, lows, volumes },
      options
    );
    const skipForReversal = /* @__PURE__ */ new Set([
      ...filteredBuy,
      ...strictBuy,
      ...trendBuy,
      ...breakoutBuy
    ]);
    const reversalBuy = computeReversalSignals(
      { closes, opens, highs, lows, volumes },
      base,
      skipForReversal,
      options
    );
    const skipForRebound = /* @__PURE__ */ new Set([
      ...filteredBuy,
      ...strictBuy,
      ...trendBuy,
      ...reversalBuy,
      ...breakoutBuy
    ]);
    const reboundBuy = computeReboundAfterSell(
      { closes, opens, highs, lows },
      base,
      skipForRebound,
      {
        sellLookback: options.reboundSellLookback ?? 6,
        minReboundPct: options.reboundMinPct ?? 0.05,
        minBodyPct: options.reboundMinBodyPct ?? 0.015,
        maxRsi: options.reboundMaxRsi ?? 60,
        minGap: options.reboundMinGap ?? 6,
        buyIndices: filteredBuy,
        buyAdjacentDays: options.reboundBuyAdjacentDays ?? 2,
        minDaysAfterSell: options.reboundMinDaysAfterSell ?? 1,
        maxUpperWickRatio: options.reboundMaxUpperWickRatio ?? 0.5,
        confirmDays: options.reboundConfirmDays ?? 2
      }
    );
    const last = len - 1;
    let latestStatus = { ...base.latestStatus };
    if (last >= 0) {
      const rl = base.rsi[last];
      const overbought = options.overbought ?? 70;
      if (rl != null && rl >= iceThreshold && rl <= overbought) {
        let daysSinceBuy = null;
        for (let i = last; i >= 0; i--) {
          if (filteredBuy.includes(i)) {
            daysSinceBuy = last - i;
            break;
          }
        }
        if (daysSinceBuy != null && daysSinceBuy <= 15) {
          latestStatus = {
            ...latestStatus,
            text: `\u51FA\u51B0\u70B9 ${daysSinceBuy} \u4E2A\u4EA4\u6613\u65E5\u524D \xB7 RSI ${rl.toFixed(1)}`,
            type: "success"
          };
        } else if (String(base.latestStatus?.text || "").includes("\u51FA\u51B0\u70B9")) {
          latestStatus = {
            ...latestStatus,
            text: `\u5E38\u6001 \xB7 RSI ${rl.toFixed(1)}`,
            type: "default"
          };
        }
      }
      const recentStrict = strictBuy.filter((i) => last - i <= 15).pop();
      const recentTrend = trendBuy.filter((i) => last - i <= 15).pop();
      const recentReversal = reversalBuy.filter((i) => last - i <= 15).pop();
      const recentBreakout = breakoutBuy.filter((i) => last - i <= 15).pop();
      const recentRebound = reboundBuy.filter((i) => last - i <= 15).pop();
      if (recentStrict != null) {
        const days = last - recentStrict;
        latestStatus = {
          ...latestStatus,
          text: `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u5F3A\u5316\u4E70\u70B9 \xB7 RSI ${base.latestStatus.rsi?.toFixed(1) ?? "\u2014"}`,
          type: "success"
        };
      } else if (recentTrend != null) {
        const days = last - recentTrend;
        const trendConfirmNote = options.trendRequireNextDayYang ? " \xB7 \u6B21\u65E5\u786E\u8BA4" : "";
        latestStatus = {
          ...latestStatus,
          text: `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u8D8B\u52BF\u4E70\u70B9${trendConfirmNote} \xB7 RSI ${base.latestStatus.rsi?.toFixed(1) ?? "\u2014"}`,
          type: "success"
        };
      } else if (recentReversal != null) {
        const days = last - recentReversal;
        latestStatus = {
          ...latestStatus,
          text: `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u8F6C\u52BF\u4E70\u70B9 \xB7 RSI ${base.latestStatus.rsi?.toFixed(1) ?? "\u2014"}`,
          type: "success"
        };
      } else if (recentBreakout != null) {
        const days = last - recentBreakout;
        latestStatus = {
          ...latestStatus,
          text: `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u5E73\u53F0\u7A81\u7834 \xB7 RSI ${base.latestStatus.rsi?.toFixed(1) ?? "\u2014"}`,
          type: "success"
        };
      } else if (recentRebound != null) {
        const days = last - recentRebound;
        latestStatus = {
          ...latestStatus,
          text: `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u5356\u540E\u7AD9\u7A33MA20 \xB7 RSI ${base.latestStatus.rsi?.toFixed(1) ?? "\u2014"}`,
          type: "warning"
        };
      }
    }
    const sellProtectByType = resolveSellProtectDays(options);
    const entryGroups = collectEntryBarGroups(filteredBuy, strictBuy, trendBuy, reversalBuy, breakoutBuy, reboundBuy);
    const entryMeta = buildEntryMetaList(entryGroups, lows, closes, sellProtectByType);
    const entryLowLookback = options.sellBreakEntryLowLookback ?? options.recentBuyDays ?? 15;
    let sellMa20 = [...base.sellMa20 || []];
    sellMa20 = augmentEarlyReduceSignals(
      sellMa20,
      { closes, opens, highs, lows },
      base.ma20,
      options
    );
    if (options.sellBreakEntryLowEnablesReduce !== false) {
      sellMa20 = augmentReduceByEntryLowBreak(sellMa20, closes, entryMeta, entryLowLookback);
    }
    sellMa20 = filterSellSignalsAfterEntry(sellMa20, entryMeta, closes, {
      sellKind: "reduce",
      sellBreakEntryLowEnablesReduce: options.sellBreakEntryLowEnablesReduce !== false
    });
    sellMa20 = dedupeSignalIndices(sellMa20, options.sellReduceMinGap ?? 8);
    let sellRsi = dedupeSignalIndices(
      filterSellSignalsAfterEntry(base.sellRsi || [], entryMeta, closes, {
        sellKind: "takeProfit",
        sellBreakEntryLowEnablesReduce: false
      }),
      options.sellTakeProfitMinGap ?? 8
    );
    sellRsi = filterTakeProfitWithProfitGate(sellRsi, entryMeta, closes, options);
    return {
      ...base,
      buy: filteredBuy,
      ma5,
      ma10,
      strictBuy,
      trendBuy,
      reversalBuy,
      breakoutBuy,
      reboundBuy,
      sellRsi,
      sellMa20,
      latestStatus
    };
  }
  function findRecentSignalBar(sig, lastIndex, tag, options = {}) {
    if (!sig || lastIndex < 0 || !tag) return null;
    const recentBuyDays = options.recentBuyDays ?? 15;
    const recentSellDays = options.recentSellDays ?? 5;
    const strongConfirmDays = options.strongConfirmDays ?? 2;
    const breakoutConfirmDays = options.breakoutConfirmDays ?? options.confirmDays ?? 3;
    function withinRecent(barIdx) {
      const daysAgo = lastIndex - barIdx;
      return daysAgo >= 0 && daysAgo <= recentBuyDays;
    }
    if (tag === SELL_TAG_REDUCE) {
      for (let i = lastIndex; i >= Math.max(0, lastIndex - recentSellDays); i--) {
        if ((sig.sellMa20 || []).includes(i)) {
          return { index: i, tag: SELL_TAG_REDUCE, daysAgo: lastIndex - i };
        }
      }
      return null;
    }
    if (tag === SELL_TAG_TAKE_PROFIT) {
      const reduceSet = new Set(sig.sellMa20 || []);
      for (let i = lastIndex; i >= Math.max(0, lastIndex - recentSellDays); i--) {
        if ((sig.sellRsi || []).includes(i) && !reduceSet.has(i)) {
          return { index: i, tag: SELL_TAG_TAKE_PROFIT, daysAgo: lastIndex - i };
        }
      }
      return null;
    }
    if (tag === "\u5356") {
      const reduce = findRecentSignalBar(sig, lastIndex, SELL_TAG_REDUCE, options);
      if (reduce) return reduce;
      return findRecentSignalBar(sig, lastIndex, SELL_TAG_TAKE_PROFIT, options);
    }
    if (tag === "\u5F3A") {
      for (let i = (sig.strictBuy || []).length - 1; i >= 0; i--) {
        const buyIdx = sig.strictBuy[i];
        const confirmedAt = buyIdx + strongConfirmDays;
        if (withinRecent(confirmedAt)) {
          return { index: buyIdx, tag: "\u5F3A", daysAgo: lastIndex - confirmedAt, confirmedAt };
        }
      }
      return null;
    }
    if (tag === "\u8D8B") {
      for (let i = (sig.trendBuy || []).length - 1; i >= 0; i--) {
        const idx = sig.trendBuy[i];
        if (withinRecent(idx)) {
          return { index: idx, tag: "\u8D8B", daysAgo: lastIndex - idx };
        }
      }
      return null;
    }
    if (tag === "\u8F6C") {
      for (let i = (sig.reversalBuy || []).length - 1; i >= 0; i--) {
        const idx = sig.reversalBuy[i];
        if (withinRecent(idx)) {
          return { index: idx, tag: "\u8F6C", daysAgo: lastIndex - idx };
        }
      }
      return null;
    }
    if (tag === "\u7A81") {
      for (let i = (sig.breakoutBuy || []).length - 1; i >= 0; i--) {
        const breakIdx = sig.breakoutBuy[i];
        const confirmedAt = breakIdx + breakoutConfirmDays;
        if (withinRecent(confirmedAt)) {
          return { index: breakIdx, tag: "\u7A81", daysAgo: lastIndex - confirmedAt, confirmedAt };
        }
      }
      return null;
    }
    if (tag === "\u5F39") {
      for (let i = (sig.reboundBuy || []).length - 1; i >= 0; i--) {
        const idx = sig.reboundBuy[i];
        if (withinRecent(idx)) {
          return { index: idx, tag: "\u5F39", daysAgo: lastIndex - idx };
        }
      }
      return null;
    }
    if (tag === "\u4E70") {
      for (let i = (sig.buy || []).length - 1; i >= 0; i--) {
        const idx = sig.buy[i];
        if (withinRecent(idx)) {
          return { index: idx, tag: "\u4E70", daysAgo: lastIndex - idx };
        }
      }
      return null;
    }
    return null;
  }
  function pickPrimaryRecentSignal(sig, lastIndex, options = {}) {
    const includeSell = options.includeSell !== false;
    for (const tag of SIGNAL_TAG_PRIORITY) {
      if (!includeSell && (tag === SELL_TAG_REDUCE || tag === SELL_TAG_TAKE_PROFIT)) continue;
      const hit = findRecentSignalBar(sig, lastIndex, tag, options);
      if (hit) return hit;
    }
    return null;
  }
  function calcSignalScore(summary) {
    if (!summary?.ok || !summary.tag) return null;
    const baseByTag = { \u5F3A: 92, \u8D8B: 84, \u52A0: 86, \u8F6C: 80, \u7A81: 78, \u5F39: 72, \u4E70: 68, \u51B0: 52, \u51CF: 16, \u6B62: 28, \u51B2: 22, \u5356: 22 };
    const base = baseByTag[summary.tag];
    if (base == null) return null;
    let score = base;
    const days = summary.recentSignalDaysAgo;
    if (days != null && days > 0) {
      score -= Math.min(12, days * 3);
    }
    const rsi = summary.latestStatus?.rsi;
    if (Number.isFinite(rsi) && !isSellSignalTag(summary.tag) && summary.tag !== "\u5356") {
      if (rsi < 25) score += 8;
      else if (rsi < 30) score += 5;
      else if (rsi < 35) score += 2;
    }
    return Math.max(0, Math.min(100, Math.round(score)));
  }
  function summarizeBuySignal(bars, options = {}) {
    const recentBuyDays = options.recentBuyDays ?? 15;
    const recentSellDays = options.recentSellDays ?? 5;
    const includeSell = options.includeSell !== false;
    const strongConfirmDays = options.strongConfirmDays ?? 2;
    const breakoutConfirmDays = options.breakoutConfirmDays ?? options.confirmDays ?? 3;
    const payload = Array.isArray(bars) ? {
      closes: bars,
      opens: [],
      highs: [],
      lows: [],
      volumes: [],
      dayKeys: [],
      indexMa20ByDay: options.indexMa20ByDay ?? null
    } : {
      closes: bars?.closes ?? [],
      opens: bars?.opens ?? [],
      highs: bars?.highs ?? [],
      lows: bars?.lows ?? [],
      volumes: bars?.volumes ?? [],
      dayKeys: bars?.dayKeys ?? [],
      indexMa20ByDay: options.indexMa20ByDay ?? bars?.indexMa20ByDay ?? null
    };
    const sig = computeFullSignals(payload, options);
    const closes = payload.closes;
    let last = closes.length - 1;
    if (options.signalLastIndex != null && options.signalLastIndex >= 0) {
      last = Math.min(options.signalLastIndex, last);
    }
    const primary = pickPrimaryRecentSignal(sig, last, {
      recentBuyDays,
      recentSellDays,
      includeSell,
      strongConfirmDays,
      breakoutConfirmDays
    });
    const recentStrict = primary?.tag === "\u5F3A" ? primary.index : null;
    const recentTrend = primary?.tag === "\u8D8B" ? primary.index : null;
    const recentReversal = primary?.tag === "\u8F6C" ? primary.index : null;
    const recentBreakout = primary?.tag === "\u7A81" ? primary.index : null;
    const recentRebound = primary?.tag === "\u5F39" ? primary.index : null;
    const recentBuy = primary?.tag === "\u4E70" ? primary.index : null;
    const recentReduce = includeSell && primary?.tag === SELL_TAG_REDUCE ? primary.index : null;
    const recentTakeProfit = includeSell && primary?.tag === SELL_TAG_TAKE_PROFIT ? primary.index : null;
    const recentSell = recentReduce ?? recentTakeProfit;
    const inIce = sig.latestStatus?.type === "info";
    let tag = primary?.tag || "";
    let tagType = "default";
    let sellPositionPct = null;
    let statusText = sig.latestStatus?.text || "\u2014";
    if (includeSell && recentReduce != null) {
      const days = primary?.daysAgo ?? last - recentReduce;
      tag = SELL_TAG_REDUCE;
      tagType = "error";
      sellPositionPct = calcSellPositionPct(tag, sig, recentReduce, payload, options);
      const hint = sellPositionHint(tag, sellPositionPct);
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u51CF \xB7 \u7834 MA20 \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}${hint ? ` \xB7 ${hint}` : ""}`;
    } else if (includeSell && recentTakeProfit != null) {
      const days = primary?.daysAgo ?? last - recentTakeProfit;
      tag = SELL_TAG_TAKE_PROFIT;
      tagType = "warning";
      sellPositionPct = calcSellPositionPct(tag, sig, recentTakeProfit, payload, options);
      const hint = sellPositionHint(tag, sellPositionPct);
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u6B62 \xB7 RSI \u56DE\u843D \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}${hint ? ` \xB7 ${hint}` : ""}`;
    } else if (recentStrict != null) {
      tag = "\u5F3A";
      tagType = "success";
      const days = primary?.daysAgo ?? last - recentStrict;
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u5F3A\u5316\u4E70\u70B9 \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}`;
    } else if (recentTrend != null) {
      tag = "\u8D8B";
      tagType = "warning";
      const days = primary?.daysAgo ?? last - recentTrend;
      const trendConfirmNote = options.trendRequireNextDayYang ? " \xB7 \u6B21\u65E5\u786E\u8BA4" : "";
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u8D8B\u52BF\u4E70\u70B9${trendConfirmNote} \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}`;
    } else if (recentReversal != null) {
      tag = "\u8F6C";
      tagType = "success";
      const days = primary?.daysAgo ?? last - recentReversal;
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u8F6C\u52BF\u4E70\u70B9 \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}`;
    } else if (recentBreakout != null) {
      tag = "\u7A81";
      tagType = "success";
      const days = primary?.daysAgo ?? last - recentBreakout;
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u5E73\u53F0\u7A81\u7834 \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}`;
    } else if (recentRebound != null) {
      tag = "\u5F39";
      tagType = "warning";
      const days = primary?.daysAgo ?? last - recentRebound;
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u5356\u540E\u7AD9\u7A33MA20 \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}`;
    } else if (recentBuy != null) {
      tag = "\u4E70";
      tagType = "success";
      const days = primary?.daysAgo ?? last - recentBuy;
      statusText = `${days === 0 ? "\u4ECA\u65E5" : `${days}\u65E5\u524D`}\u51FA\u51B0\u70B9\u4E70\u70B9 \xB7 RSI ${sig.latestStatus?.rsi?.toFixed(1) ?? "\u2014"}`;
    } else if (inIce) {
      tag = "\u51B0";
      tagType = "info";
      statusText = sig.latestStatus?.text || "\u51B0\u70B9\u533A\u5185";
    }
    const summary = {
      ...sig,
      ok: true,
      hasRecentBuy: recentStrict != null || recentTrend != null || recentReversal != null || recentBreakout != null || recentRebound != null || recentBuy != null,
      hasRecentStrictBuy: recentStrict != null,
      hasRecentTrendBuy: recentTrend != null,
      hasRecentReversal: recentReversal != null,
      hasRecentBreakout: recentBreakout != null,
      hasRecentRebound: recentRebound != null,
      hasRecentBuyBasic: recentBuy != null,
      hasRecentSell: recentSell != null,
      hasRecentReduce: recentReduce != null,
      hasRecentTakeProfit: recentTakeProfit != null,
      signalLastIndex: last,
      effectiveSignalDayKey: payload.dayKeys?.[last] ? normalizeDayKey(payload.dayKeys[last]) : "",
      recentSignalBar: primary?.index ?? null,
      recentSignalConfirmBar: primary?.confirmedAt ?? primary?.index ?? null,
      recentSignalDaysAgo: primary?.daysAgo ?? null,
      inIce,
      tag,
      tagType,
      sellPositionPct,
      statusText,
      sortRank: signalSortRank(tag || (inIce ? "\u51B0" : ""))
    };
    summary.signalScore = calcSignalScore(summary);
    summary.buyPriceRange = calcBuyPriceRange(summary, payload, options);
    return summary;
  }

  // src/utils/quantAutomationSettings.js
  var DEFAULT_QUANT_AUTOMATION = {
    enabled: false,
    /** 账户总权益（元），用于仓位计算 */
    accountEquity: 5e5,
    /** 单笔最大风险占权益比例 */
    riskPerTradePct: 0.01,
    /** 单票最大仓位占权益比例 */
    maxPositionPct: 0.15,
    /** 组合最大总敞口占权益比例 */
    maxTotalExposurePct: 0.85,
    /** A 股最小交易单位 */
    minLotSize: 100,
    /** 信号置信度 → 仓位系数 */
    tagConfidence: {
      \u5F3A: 1,
      \u7A81: 0.85,
      \u8D8B: 0.75,
      \u8F6C: 0.65,
      \u5F39: 0.65,
      \u4E70: 0.5
    },
    alerts: {
      signalNew: true,
      zoneTouch: true,
      zoneLeave: false,
      sellSignal: true,
      sellDraft: true,
      checklistReady: true,
      positionPlan: true
    },
    /** 同类告警冷却（分钟） */
    alertCooldownMinutes: 15,
    /** 区间触达容差 */
    zoneTouchTolerancePct: 3e-3,
    /** 信号扫描间隔（分钟） */
    scanIntervalMinutes: 20,
    /** 清单就绪最低得分（0~1） */
    checklistReadyScore: 0.85,
    /** 清单必须项全部通过才推送 */
    checklistRequireRequired: true
  };
  function mergeQuantAutomation(raw) {
    const base = JSON.parse(JSON.stringify(DEFAULT_QUANT_AUTOMATION));
    if (!raw || typeof raw !== "object") return base;
    Object.assign(base, raw);
    if (raw.tagConfidence) base.tagConfidence = { ...base.tagConfidence, ...raw.tagConfidence };
    if (raw.alerts) base.alerts = { ...base.alerts, ...raw.alerts };
    return base;
  }

  // src/utils/signalSettings.js
  var DEFAULT_SCREEN_STRATEGY_ID = "default";
  var DEFAULT_SCREEN_STRATEGY_NAME = "\u9ED8\u8BA4\u7B56\u7565";
  var DEFAULT_SIGNAL_SETTINGS = {
    automation: DEFAULT_QUANT_AUTOMATION,
    display: {
      watchlistCardSignalTint: false,
      watchlistActionPopup: false,
      followDateGroupEnabled: true,
      dateGroupRetainDays: 30,
      recentBuyDays: 15,
      recentSellDays: 5,
      /** 自选股票数量上限 */
      maxFollowCount: 100
    },
    common: {
      rsiPeriod: 14,
      iceThreshold: 30,
      overbought: 70,
      lookback: 5,
      maPeriod: 20,
      volPeriod: 5,
      requireIndexBull: true
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
      protectAfterEntryDays: 0
    },
    buy: {
      minBodyPct: 0.01
    },
    strong: {
      volMult: 1.25,
      confirmDays: 2,
      iceMinGap: 4,
      requireStockAboveMa20: true,
      requireMa20Rising: true
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
      requireNextDayYang: true
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
      minGap: 5
    },
    breakout: {
      boxPeriod: 20,
      maxRangePct: 0.2,
      volMult: 1.25,
      minGap: 15,
      breakBuffer: 5e-3,
      confirmDays: 2,
      maxUpperWickRatio: 0.55
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
      screenMaxRsi: 60
    }
  };
  function cloneDefaultSignalSettingsCore() {
    const base = deepClone(DEFAULT_SIGNAL_SETTINGS);
    delete base.automation;
    delete base.display;
    delete base.activeScreenStrategyId;
    delete base.screenStrategies;
    return base;
  }
  var SIGNAL_PARAM_SECTIONS = [
    {
      key: "common",
      title: "\u901A\u7528",
      groups: [
        { id: "core", title: "RSI / \u5747\u7EBF", desc: "\u51B0\u3001\u4E70\u3001\u6B62\u3001\u51CF\u5171\u7528\u7684\u57FA\u7840\u6307\u6807" },
        { id: "env", title: "\u5F3A\u4E70\u73AF\u5883" }
      ],
      fields: [
        { key: "rsiPeriod", label: "RSI \u5468\u671F", group: "core", type: "int", min: 5, max: 30, step: 1 },
        { key: "iceThreshold", label: "\u51B0\u70B9\u9608\u503C", group: "core", type: "number", min: 10, max: 45, step: 1 },
        { key: "overbought", label: "\u6B62 \xB7 RSI \u9608\u503C", group: "core", type: "number", min: 60, max: 90, step: 1 },
        { key: "lookback", label: "\u51FA\u51B0\u56DE\u6EAF", group: "core", type: "int", min: 1, max: 20, step: 1, suffix: "\u65E5" },
        { key: "maPeriod", label: "\u51CF \xB7 MA \u5468\u671F", group: "core", type: "int", min: 5, max: 60, step: 1 },
        { key: "volPeriod", label: "\u5747\u91CF\u5468\u671F", group: "core", type: "int", min: 3, max: 20, step: 1 },
        { key: "requireIndexBull", label: "\u5F3A\u4E70\u987B\u4E0A\u8BC1 MA20 \u4E0A", group: "env", type: "bool" }
      ]
    },
    {
      key: "sell",
      title: "\u6B62 / \u51CF",
      groups: [
        { id: "reduce", title: "\u51CF \xB7 \u4E3B\u89C4\u5219", desc: "\u7ED3\u6784\u8D70\u574F\u65F6\u6807\u51CF\uFF08\u4E3B\u5356\u4FE1\u53F7\uFF09" },
        { id: "reduceEarly", title: "\u51CF \xB7 \u63D0\u524D\u9884\u8B66", desc: "MA20 \u672A\u7834\u524D\u7684\u8D70\u5F31\uFF1B\u53EF\u6574\u4F53\u5173\u95ED" },
        { id: "takeProfit", title: "\u6B62 \xB7 \u6B62\u76C8", desc: "\u6709\u6D6E\u76C8\u65F6\u7684\u8F85\u52A9\u6B62\u76C8\uFF0C\u4E0D\u5F53\u6E05\u4ED3\u4F9D\u636E" },
        { id: "protect", title: "\u4E70\u70B9\u540E\u4FDD\u62A4", desc: "\u5404\u4E70\u70B9\u540E N \u65E5\u5185\u9ED8\u8BA4\u4E0D\u6807\u6B62/\u51CF\uFF1B0 = \u5173\u95ED" }
      ],
      fields: [
        {
          key: "reduceConfirmDays",
          label: "\u8FDE\u7EED\u7834 MA20",
          group: "reduce",
          type: "int",
          min: 1,
          max: 5,
          step: 1,
          suffix: "\u65E5",
          hint: "1 = \u9996\u6B21\u8DCC\u7834\u5F53\u65E5"
        },
        { key: "breakEntryLowEnablesReduce", label: "\u7834\u4E70\u70B9\u4F4E\u70B9\u4ECD\u6807", group: "reduce", type: "bool" },
        { key: "breakEntryLowLookback", label: "\u7834\u4F4E\u70B9\u56DE\u6EAF", group: "reduce", type: "int", min: 5, max: 30, step: 1, suffix: "\u65E5" },
        { key: "reduceMinGap", label: "\u540C\u80A1\u95F4\u9694", group: "reduce", type: "int", min: 0, max: 20, step: 1, suffix: "\u65E5" },
        { key: "reduceEarlyEnabled", label: "\u542F\u7528\u63D0\u524D\u9884\u8B66", group: "reduceEarly", type: "bool" },
        {
          key: "reduceEarlyMa5Days",
          label: "\u8FDE\u7EED\u7834 MA5",
          group: "reduceEarly",
          type: "int",
          min: 1,
          max: 4,
          step: 1,
          suffix: "\u65E5"
        },
        {
          key: "reduceEarlyMinDropPct",
          label: "\u5927\u9634\u8DCC\u5E45",
          group: "reduceEarly",
          type: "number",
          min: 0.02,
          max: 0.1,
          step: 5e-3,
          scale: 100,
          suffix: "%",
          uiPrecision: 1
        },
        {
          key: "reduceEarlyBreakLowDays",
          label: "\u7834\u8FD1\u7AEF\u4F4E\u70B9",
          group: "reduceEarly",
          type: "int",
          min: 2,
          max: 10,
          step: 1,
          suffix: "\u65E5"
        },
        {
          key: "takeProfitMinOverboughtDays",
          label: "RSI \u8D85\u4E70\u505C\u7559",
          group: "takeProfit",
          type: "int",
          min: 1,
          max: 5,
          step: 1,
          suffix: "\u65E5"
        },
        { key: "takeProfitMinGap", label: "\u540C\u80A1\u95F4\u9694", group: "takeProfit", type: "int", min: 0, max: 20, step: 1, suffix: "\u65E5" },
        { key: "sellProfitGateEnabled", label: "\u987B\u6EE1\u8DB3\u6D6E\u76C8\u95E8\u69DB", group: "takeProfit", type: "bool" },
        { key: "takeProfitRequirePullbackFromPeak", label: "\u987B\u81EA\u9AD8\u70B9\u56DE\u64A4", group: "takeProfit", type: "bool" },
        {
          key: "takeProfitMinGainPct",
          label: "\u6700\u4F4E\u6D6E\u76C8",
          group: "takeProfit",
          type: "number",
          min: 0.02,
          max: 0.2,
          step: 0.01,
          scale: 100,
          suffix: "%",
          uiPrecision: 0
        },
        {
          key: "takeProfitMinPullbackFromPeakPct",
          label: "\u81EA\u9AD8\u70B9\u56DE\u64A4",
          group: "takeProfit",
          type: "number",
          min: 0.02,
          max: 0.15,
          step: 5e-3,
          scale: 100,
          suffix: "%",
          uiPrecision: 1
        },
        { key: "protectAfterTrend", label: "\u8D8B", group: "protect", type: "int", min: 0, max: 10, step: 1, suffix: "\u65E5" },
        { key: "protectAfterStrong", label: "\u5F3A", group: "protect", type: "int", min: 0, max: 10, step: 1, suffix: "\u65E5" },
        { key: "protectAfterBreakout", label: "\u7A81", group: "protect", type: "int", min: 0, max: 10, step: 1, suffix: "\u65E5" },
        { key: "protectAfterReversal", label: "\u8F6C", group: "protect", type: "int", min: 0, max: 10, step: 1, suffix: "\u65E5" },
        { key: "protectAfterBuy", label: "\u4E70", group: "protect", type: "int", min: 0, max: 10, step: 1, suffix: "\u65E5" },
        { key: "protectAfterRebound", label: "\u5F39", group: "protect", type: "int", min: 0, max: 10, step: 1, suffix: "\u65E5" }
      ]
    },
    {
      key: "strong",
      title: "\u5F3A",
      fields: [
        { key: "volMult", label: "\u653E\u91CF\u500D\u6570", type: "number", min: 1, max: 3, step: 0.05 },
        { key: "confirmDays", label: "\u786E\u8BA4\u7AD9\u7A33\u5929\u6570", type: "int", min: 0, max: 5, step: 1, suffix: "\u65E5" },
        { key: "iceMinGap", label: "\u4E0E\u51B0\u70B9\u6700\u5C0F\u95F4\u9694", type: "int", min: 1, max: 15, step: 1, suffix: "\u65E5" },
        { key: "requireStockAboveMa20", label: "\u987B\u6536\u5728 MA20 \u4E0A", type: "bool" },
        { key: "requireMa20Rising", label: "\u987B MA20 \u4E0A\u884C", type: "bool" }
      ]
    },
    {
      key: "trend",
      title: "\u8D8B",
      groups: [
        { id: "entry", title: "\u56DE\u8E29\u6761\u4EF6" },
        { id: "chop", title: "\u9707\u8361\u8FC7\u6EE4", desc: "\u7C98\u5408 / \u8FD1\u7AEF\u6B62\u51CF / \u6A2A\u76D8\u65F6\u4E0D\u6807\u8D8B" }
      ],
      fields: [
        { key: "rsiMin", label: "RSI \u4E0B\u9650", group: "entry", type: "number", min: 30, max: 60, step: 1 },
        { key: "rsiMax", label: "RSI \u4E0A\u9650", group: "entry", type: "number", min: 55, max: 80, step: 1 },
        { key: "touchMaPct", label: "\u56DE\u8E29 MA \u5BB9\u5DEE", group: "entry", type: "number", min: 5e-3, max: 0.03, step: 1e-3, scale: 100, suffix: "%", uiPrecision: 1 },
        {
          key: "maxCloseAboveMa5Pct",
          label: "\u6536\u76D8\u8DDD MA5 \u4E0A\u9650",
          group: "entry",
          type: "number",
          min: 0,
          max: 0.15,
          step: 5e-3,
          scale: 100,
          suffix: "%",
          uiPrecision: 1,
          hint: "0 = \u4E0D\u9650\u5236"
        },
        { key: "minUpBars", label: "MA20 \u4E0A\u65B9\u6700\u5C11", group: "entry", type: "int", min: 1, max: 5, step: 1, suffix: "\u65E5" },
        { key: "minGap", label: "\u540C\u80A1\u95F4\u9694", group: "entry", type: "int", min: 1, max: 15, step: 1, suffix: "\u65E5" },
        {
          key: "requireNextDayYang",
          label: "\u987B\u6B21\u65E5\u6536\u9633\u786E\u8BA4",
          group: "entry",
          type: "bool",
          hint: "\u5F00\u542F\u540E\u5728\u6B21\u65E5 K \u7EBF\u6807\u8D8B\uFF1B\u5173\u95ED\u5219\u56DE\u8E29\u6536\u9633\u5F53\u65E5\u5373\u6807"
        },
        { key: "chopFilterEnabled", label: "\u542F\u7528\u9707\u8361\u8FC7\u6EE4", group: "chop", type: "bool" },
        { key: "minMaSpreadPct", label: "MA5-MA10 \u95F4\u8DDD", group: "chop", type: "number", min: 3e-3, max: 0.03, step: 1e-3, scale: 100, suffix: "%", uiPrecision: 1 },
        { key: "chopSellLookback", label: "\u6B62/\u51CF\u56DE\u6EAF", group: "chop", type: "int", min: 0, max: 20, step: 1, suffix: "\u65E5", hint: "0 = \u4E0D\u68C0\u67E5" },
        { key: "chopRangeDays", label: "\u6A2A\u76D8\u89C2\u5BDF", group: "chop", type: "int", min: 3, max: 10, step: 1, suffix: "\u65E5" },
        { key: "chopMaxRangePct", label: "\u6A2A\u76D8\u632F\u5E45\u4E0A\u9650", group: "chop", type: "number", min: 0.04, max: 0.15, step: 5e-3, scale: 100, suffix: "%", uiPrecision: 1 }
      ]
    },
    {
      key: "reversal",
      title: "\u8F6C",
      groups: [
        { id: "core", title: "\u8F6C\u52BF\u89E6\u53D1" },
        { id: "filter", title: "\u8FC7\u6EE4\u6761\u4EF6" }
      ],
      fields: [
        { key: "enabled", label: "\u542F\u7528\u8F6C\u52BF\u4FE1\u53F7", group: "core", type: "bool" },
        { key: "crossMaPeriod", label: "\u4E0A\u7A7F\u5747\u7EBF\u5468\u671F", group: "core", type: "int", min: 20, max: 120, step: 5, suffix: "\u65E5" },
        { key: "minBodyPct", label: "\u9633\u5B9E\u4F53\u4E0B\u9650", group: "core", type: "number", min: 5e-3, max: 0.05, step: 1e-3, scale: 100, suffix: "%", uiPrecision: 1 },
        { key: "minGap", label: "\u540C\u80A1\u95F4\u9694", group: "core", type: "int", min: 3, max: 20, step: 1, suffix: "\u65E5" },
        { key: "rsiMin", label: "RSI \u4E0B\u9650", group: "filter", type: "number", min: 30, max: 60, step: 1 },
        { key: "rsiMax", label: "RSI \u4E0A\u9650", group: "filter", type: "number", min: 55, max: 80, step: 1 },
        { key: "requireMa20FlatOrUp", label: "MA20 \u8D70\u5E73/\u5411\u4E0A", group: "filter", type: "bool" },
        { key: "ma20LookbackDays", label: "MA20 \u5BF9\u6BD4\u56DE\u6EAF", group: "filter", type: "int", min: 3, max: 20, step: 1, suffix: "\u65E5" },
        { key: "requireCloseAboveMa5", label: "\u6536\u76D8 > MA5", group: "filter", type: "bool" },
        { key: "requireMa5AboveMa10", label: "MA5 > MA10", group: "filter", type: "bool" },
        { key: "recentBelowDays", label: "\u5747\u7EBF\u4E0B\u89C2\u5BDF", group: "filter", type: "int", min: 3, max: 15, step: 1, suffix: "\u65E5" },
        { key: "recentBelowMinRatio", label: "\u5747\u7EBF\u4E0B\u5360\u6BD4", group: "filter", type: "number", min: 0.4, max: 1, step: 0.1, scale: 100, suffix: "%", uiPrecision: 0 }
      ]
    },
    {
      key: "breakout",
      title: "\u7A81",
      fields: [
        { key: "boxPeriod", label: "\u7BB1\u4F53\u89C2\u5BDF\u5468\u671F", type: "int", min: 10, max: 40, step: 1, suffix: "\u65E5" },
        { key: "maxRangePct", label: "\u7BB1\u4F53\u6700\u5927\u632F\u5E45", type: "number", min: 0.08, max: 0.35, step: 0.01, scale: 100, suffix: "%" },
        { key: "volMult", label: "\u7A81\u7834\u653E\u91CF\u500D\u6570", type: "number", min: 1, max: 3, step: 0.05 },
        { key: "minGap", label: "\u540C\u80A1\u91CD\u590D\u95F4\u9694", type: "int", min: 5, max: 30, step: 1, suffix: "\u65E5" },
        { key: "breakBuffer", label: "\u7A81\u7834\u7F13\u51B2", type: "number", min: 0, max: 0.02, step: 1e-3, scale: 100, suffix: "%" },
        { key: "confirmDays", label: "\u7A81\u7834\u786E\u8BA4\u5929\u6570", type: "int", min: 0, max: 5, step: 1, suffix: "\u65E5" },
        { key: "maxUpperWickRatio", label: "\u4E0A\u5F71\u7EBF\u5360\u632F\u5E45\u4E0A\u9650", type: "number", min: 0.3, max: 0.8, step: 0.05, scale: 100, suffix: "%" }
      ]
    },
    {
      key: "rebound",
      title: "\u5F39",
      fields: [
        { key: "sellLookback", label: "\u6B62/\u51CF\u56DE\u6EAF\u7A97\u53E3", type: "int", min: 3, max: 15, step: 1, suffix: "\u65E5" },
        { key: "minDaysAfterSell", label: "\u8DDD\u6B62/\u51CF\u6700\u5C11\u95F4\u9694", type: "int", min: 0, max: 5, step: 1, suffix: "\u65E5" },
        { key: "minBodyPct", label: "\u9633\u5B9E\u4F53\u4E0B\u9650", type: "number", min: 5e-3, max: 0.05, step: 1e-3, scale: 100, suffix: "%" },
        { key: "maxRsi", label: "RSI \u4E0A\u9650", type: "number", min: 50, max: 75, step: 1 },
        { key: "minReboundPct", label: "\u81EA\u4F4E\u70B9\u53CD\u5F39 (\u8DEF\u5F84 B)", type: "number", min: 0.02, max: 0.15, step: 0.01, scale: 100, suffix: "%" },
        { key: "buyAdjacentDays", label: "\u4E0E\u300C\u4E70\u300D\u76F8\u90BB\u6392\u9664", type: "int", min: 0, max: 5, step: 1, suffix: "\u65E5" },
        { key: "minGap", label: "\u540C\u80A1\u91CD\u590D\u6807\u5F39\u95F4\u9694", type: "int", min: 3, max: 15, step: 1, suffix: "\u65E5" },
        { key: "confirmDays", label: "MA20 \u786E\u8BA4\u5929\u6570", type: "int", min: 1, max: 4, step: 1, suffix: "\u65E5" },
        { key: "maxUpperWickRatio", label: "\u4E0A\u5F71\u7EBF\u5360\u632F\u5E45\u4E0A\u9650", type: "number", min: 0.3, max: 0.8, step: 0.05, scale: 100, suffix: "%" },
        { key: "screenMaxRsi", label: "\u5217\u8868\u7B5B\u9009\u6392\u9664 RSI", type: "number", min: 50, max: 75, step: 1 }
      ]
    },
    {
      key: "buy",
      title: "\u4E70",
      fields: [
        { key: "minBodyPct", label: "\u9633\u5B9E\u4F53\u4E0B\u9650", type: "number", min: 5e-3, max: 0.05, step: 1e-3, scale: 100, suffix: "%" }
      ]
    }
  ];
  function deepClone(obj) {
    return JSON.parse(JSON.stringify(obj));
  }
  function mergeSignalStrategySettings(raw) {
    const base = cloneDefaultSignalSettingsCore();
    if (!raw || typeof raw !== "object") return base;
    for (const section of SIGNAL_PARAM_SECTIONS) {
      const key = section.key;
      if (!raw[key] || typeof raw[key] !== "object") continue;
      const values = { ...raw[key] };
      if (key === "common") {
        delete values.recentBuyDays;
        delete values.recentSellDays;
      }
      base[key] = { ...base[key], ...values };
    }
    return base;
  }
  function cloneDefaultSignalSettings() {
    const base = deepClone(DEFAULT_SIGNAL_SETTINGS);
    base.activeScreenStrategyId = DEFAULT_SCREEN_STRATEGY_ID;
    base.screenStrategies = [
      {
        id: DEFAULT_SCREEN_STRATEGY_ID,
        name: DEFAULT_SCREEN_STRATEGY_NAME,
        settings: cloneDefaultSignalSettingsCore()
      }
    ];
    return base;
  }
  function mergeSignalSettings(raw) {
    const base = cloneDefaultSignalSettings();
    if (!raw || typeof raw !== "object") return base;
    if (raw.automation) base.automation = mergeQuantAutomation(raw.automation);
    if (raw.display) base.display = { ...base.display, ...raw.display };
    if (raw.common && typeof raw.common === "object") {
      if (raw.display?.recentBuyDays == null && raw.common.recentBuyDays != null) {
        base.display.recentBuyDays = raw.common.recentBuyDays;
      }
      if (raw.display?.recentSellDays == null && raw.common.recentSellDays != null) {
        base.display.recentSellDays = raw.common.recentSellDays;
      }
    }
    for (const section of SIGNAL_PARAM_SECTIONS) {
      const key = section.key;
      if (!raw[key] || typeof raw[key] !== "object") continue;
      base[key] = { ...base[key], ...raw[key] };
    }
    base.activeScreenStrategyId = String(raw.activeScreenStrategyId || base.activeScreenStrategyId || DEFAULT_SCREEN_STRATEGY_ID);
    const rawStrategies = Array.isArray(raw.screenStrategies) ? raw.screenStrategies : [];
    const strategies = rawStrategies.map((item, index) => {
      const id = String(item?.id || "").trim() || `strategy-${index + 1}`;
      const name = String(item?.name || "").trim() || `\u7B56\u7565 ${index + 1}`;
      const settings = mergeSignalStrategySettings(item?.settings || {});
      return { id, name, settings };
    }).filter((item) => item.id && item.name);
    if (strategies.length) {
      base.screenStrategies = strategies;
      if (!strategies.some((item) => item.id === base.activeScreenStrategyId)) {
        base.activeScreenStrategyId = strategies[0].id;
      }
    } else {
      base.screenStrategies = [
        {
          id: DEFAULT_SCREEN_STRATEGY_ID,
          name: DEFAULT_SCREEN_STRATEGY_NAME,
          settings: mergeSignalStrategySettings(base)
        }
      ];
      base.activeScreenStrategyId = DEFAULT_SCREEN_STRATEGY_ID;
    }
    return base;
  }
  function parseSignalParams(raw) {
    if (!raw) return cloneDefaultSignalSettings();
    try {
      const parsed = typeof raw === "string" ? JSON.parse(raw) : raw;
      return mergeSignalSettings(parsed);
    } catch {
      return cloneDefaultSignalSettings();
    }
  }
  function buildSignalOptions(settings) {
    const s = mergeSignalSettings(settings);
    const { common, buy, strong, trend, reversal, breakout, rebound, sell } = s;
    const legacyProtect = sell?.protectAfterEntryDays ?? common?.sellProtectAfterEntryDays ?? DEFAULT_SIGNAL_SETTINGS.sell.protectAfterEntryDays;
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
      reboundMaxUpperWickRatio: rebound.maxUpperWickRatio
    };
  }

  // src/utils/signalTagConstants.js
  var SCREEN_SNAPSHOT_SIGNAL_TAGS = ["\u5F3A", "\u8D8B", "\u8F6C", "\u7A81", "\u5F39", "\u4E70"];
  var SCREEN_SNAPSHOT_SIGNAL_TAG_SET = new Set(SCREEN_SNAPSHOT_SIGNAL_TAGS);

  // ../scripts/scansignals/scan-batch.ts
  function runSignalScanBatch(input) {
    const stocks = input?.stocks || [];
    const indexMa20ByDay = buildIndexMa20ByDay(new Map(Object.entries(input?.indexClose || {})), 20);
    let baseSettings = cloneDefaultSignalSettings();
    if (input?.signalParamsJson) {
      try {
        baseSettings = parseSignalParams(input.signalParamsJson);
      } catch {
      }
    }
    const options = {
      ...buildSignalOptions(baseSettings),
      recentBuyDays: 0,
      recentSellDays: 0,
      includeSell: input?.includeSell !== false
    };
    const items = [];
    for (const s of stocks) {
      const bars = {
        closes: s.closes,
        opens: s.opens,
        highs: s.highs,
        lows: s.lows,
        volumes: s.volumes,
        dayKeys: s.dayKeys,
        indexMa20ByDay
      };
      const lastIdx = s.lastBarIndex != null && s.lastBarIndex >= 0 ? Math.min(s.lastBarIndex, s.closes.length - 1) : s.closes.length - 1;
      if (lastIdx < 0) continue;
      const summary = summarizeBuySignal(bars, { ...options, signalLastIndex: lastIdx });
      if (!summary?.tag || !SCREEN_SNAPSHOT_SIGNAL_TAG_SET.has(summary.tag)) continue;
      const row = s.row || {};
      items.push({
        SECUCODE: row.SECUCODE || s.secucode || s.code,
        SECURITY_CODE: row.SECURITY_CODE || "",
        SECURITY_NAME_ABBR: row.SECURITY_NAME_ABBR || s.name || "",
        NEW_PRICE: row.NEW_PRICE ?? "",
        CHANGE_RATE: row.CHANGE_RATE ?? "",
        HIGH_PRICE: row.HIGH_PRICE ?? "",
        LOW_PRICE: row.LOW_PRICE ?? "",
        PRE_CLOSE_PRICE: row.PRE_CLOSE_PRICE ?? "",
        VOLUME: row.VOLUME ?? "",
        DEAL_AMOUNT: row.DEAL_AMOUNT ?? "",
        TURNOVERRATE: row.TURNOVERRATE ?? "",
        VOLUME_RATIO: row.VOLUME_RATIO ?? "",
        INDUSTRY: row.INDUSTRY ?? "",
        CONCEPT: row.CONCEPT ?? "",
        MARKET: row.MARKET ?? "",
        tag: summary.tag,
        recentSignalDaysAgo: summary.recentSignalDaysAgo,
        statusText: summary.statusText,
        sortRank: summary.sortRank,
        rsi: summary.latestStatus?.rsi ?? null,
        ok: true
      });
    }
    items.sort((a, b) => {
      const ra = Number(a.sortRank) || 0;
      const rb = Number(b.sortRank) || 0;
      if (ra !== rb) return rb - ra;
      return String(a.SECURITY_NAME_ABBR || "").localeCompare(String(b.SECURITY_NAME_ABBR || ""), "zh-CN");
    });
    return { items, hitTotal: items.length };
  }
  if (typeof globalThis !== "undefined") {
    ;
    globalThis.SignalScanBatch = {
      runSignalScanBatch
    };
  }
  return __toCommonJS(scan_batch_exports);
})();
