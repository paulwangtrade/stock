export namespace broker {
	
	export class OrderResult {
	    ok: boolean;
	    brokerId: string;
	    message: string;
	    rawStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new OrderResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.brokerId = source["brokerId"];
	        this.message = source["message"];
	        this.rawStatus = source["rawStatus"];
	    }
	}

}

export namespace data {
	
	export class AIConfig {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    name: string;
	    baseUrl: string;
	    apiKey: string;
	    modelName: string;
	    maxTokens: number;
	    temperature: number;
	    timeOut: number;
	    httpProxy: string;
	    httpProxyEnabled: boolean;
	    sessionId: string;
	    thinking: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.name = source["name"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.modelName = source["modelName"];
	        this.maxTokens = source["maxTokens"];
	        this.temperature = source["temperature"];
	        this.timeOut = source["timeOut"];
	        this.httpProxy = source["httpProxy"];
	        this.httpProxyEnabled = source["httpProxyEnabled"];
	        this.sessionId = source["sessionId"];
	        this.thinking = source["thinking"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AllStockInfoPageData {
	    list: models.AllStockInfo[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfoPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], models.AllStockInfo);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AllStockInfoQuery {
	    page: number;
	    pageSize: number;
	    securityCode: string;
	    securityName: string;
	    market: string;
	    industry: string;
	    concept: string;
	    minPrice: string;
	    maxPrice: string;
	    minChange: string;
	    maxChange: string;
	    searchKeyWord: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfoQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.securityCode = source["securityCode"];
	        this.securityName = source["securityName"];
	        this.market = source["market"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.minPrice = source["minPrice"];
	        this.maxPrice = source["maxPrice"];
	        this.minChange = source["minChange"];
	        this.maxChange = source["maxChange"];
	        this.searchKeyWord = source["searchKeyWord"];
	    }
	}
	export class AnalysisChainItem {
	    stockCode: string;
	    stockName: string;
	    inCandidatePool: boolean;
	    poolRank?: number;
	    score: number;
	    strategyName: string;
	    strategyVersion: string;
	    signalTag: string;
	    signalScore: number;
	    signalSnapshotId: number;
	    planPriority?: number;
	    planStatus?: string;
	    riskCode?: string;
	    riskMessage?: string;
	    targetAmount?: number;
	    orderId?: number;
	    fillId?: number;
	    filledPrice?: number;
	    filledVolume?: number;
	    filledFee?: number;
	    error?: string;
	    whyNotBought?: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisChainItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.inCandidatePool = source["inCandidatePool"];
	        this.poolRank = source["poolRank"];
	        this.score = source["score"];
	        this.strategyName = source["strategyName"];
	        this.strategyVersion = source["strategyVersion"];
	        this.signalTag = source["signalTag"];
	        this.signalScore = source["signalScore"];
	        this.signalSnapshotId = source["signalSnapshotId"];
	        this.planPriority = source["planPriority"];
	        this.planStatus = source["planStatus"];
	        this.riskCode = source["riskCode"];
	        this.riskMessage = source["riskMessage"];
	        this.targetAmount = source["targetAmount"];
	        this.orderId = source["orderId"];
	        this.fillId = source["fillId"];
	        this.filledPrice = source["filledPrice"];
	        this.filledVolume = source["filledVolume"];
	        this.filledFee = source["filledFee"];
	        this.error = source["error"];
	        this.whyNotBought = source["whyNotBought"];
	    }
	}
	export class AnalysisPlanInfo {
	    id: number;
	    status: string;
	    poolId: number;
	    riskStatus: string;
	    marketLevel: number;
	    riskAcceptedCount: number;
	    riskFilteredCount: number;
	    riskSummary: string;
	    amountPerStock: number;
	    enableExecute: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisPlanInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.poolId = source["poolId"];
	        this.riskStatus = source["riskStatus"];
	        this.marketLevel = source["marketLevel"];
	        this.riskAcceptedCount = source["riskAcceptedCount"];
	        this.riskFilteredCount = source["riskFilteredCount"];
	        this.riskSummary = source["riskSummary"];
	        this.amountPerStock = source["amountPerStock"];
	        this.enableExecute = source["enableExecute"];
	        this.message = source["message"];
	    }
	}
	export class AnalysisPoolInfo {
	    id: number;
	    source: string;
	    sourceRef: string;
	    status: string;
	    itemCount: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisPoolInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.sourceRef = source["sourceRef"];
	        this.status = source["status"];
	        this.itemCount = source["itemCount"];
	        this.message = source["message"];
	    }
	}
	export class DailyCandidateStatus {
	    poolId?: number;
	    status: string;
	    count: number;
	    source?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyCandidateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.poolId = source["poolId"];
	        this.status = source["status"];
	        this.count = source["count"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}
	export class DailyExecutionStatus {
	    phase: string;
	    ready: boolean;
	    executorConfigured: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyExecutionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.ready = source["ready"];
	        this.executorConfigured = source["executorConfigured"];
	        this.message = source["message"];
	    }
	}
	export class DailyPaperStatus {
	    orderCount: number;
	    fillCount: number;
	    filledOrderCount: number;
	    hasAccount: boolean;
	    accountId?: number;
	    accountCash?: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyPaperStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orderCount = source["orderCount"];
	        this.fillCount = source["fillCount"];
	        this.filledOrderCount = source["filledOrderCount"];
	        this.hasAccount = source["hasAccount"];
	        this.accountId = source["accountId"];
	        this.accountCash = source["accountCash"];
	    }
	}
	export class DailyPlanStatus {
	    planId?: number;
	    status: string;
	    itemCount: number;
	    pendingCount: number;
	    filledCount: number;
	    skippedCount: number;
	    errorCount: number;
	    message?: string;
	    executingSince?: string;
	    reconcileRecommended: boolean;
	    reconcileRecommendedReason?: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyPlanStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.planId = source["planId"];
	        this.status = source["status"];
	        this.itemCount = source["itemCount"];
	        this.pendingCount = source["pendingCount"];
	        this.filledCount = source["filledCount"];
	        this.skippedCount = source["skippedCount"];
	        this.errorCount = source["errorCount"];
	        this.message = source["message"];
	        this.executingSince = source["executingSince"];
	        this.reconcileRecommended = source["reconcileRecommended"];
	        this.reconcileRecommendedReason = source["reconcileRecommendedReason"];
	    }
	}
	export class DailyRiskStatus {
	    status: string;
	    acceptedCount: number;
	    filteredCount: number;
	    summary?: string;
	    marketLevel: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyRiskStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.acceptedCount = source["acceptedCount"];
	        this.filteredCount = source["filteredCount"];
	        this.summary = source["summary"];
	        this.marketLevel = source["marketLevel"];
	    }
	}
	export class DailyTradingStatus {
	    tradeDate: string;
	    isWeekday: boolean;
	    enablePaperOpenBuy: boolean;
	    candidate: DailyCandidateStatus;
	    plan: DailyPlanStatus;
	    risk: DailyRiskStatus;
	    execution: DailyExecutionStatus;
	    paper: DailyPaperStatus;
	    blockReason: string;
	    blockReasons: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DailyTradingStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tradeDate = source["tradeDate"];
	        this.isWeekday = source["isWeekday"];
	        this.enablePaperOpenBuy = source["enablePaperOpenBuy"];
	        this.candidate = this.convertValues(source["candidate"], DailyCandidateStatus);
	        this.plan = this.convertValues(source["plan"], DailyPlanStatus);
	        this.risk = this.convertValues(source["risk"], DailyRiskStatus);
	        this.execution = this.convertValues(source["execution"], DailyExecutionStatus);
	        this.paper = this.convertValues(source["paper"], DailyPaperStatus);
	        this.blockReason = source["blockReason"];
	        this.blockReasons = source["blockReasons"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FundBasic {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    code: string;
	    name: string;
	    fullName: string;
	    type: string;
	    establishment: string;
	    scale: string;
	    company: string;
	    manager: string;
	    rating: string;
	    trackingTarget: string;
	    netUnitValue?: number;
	    netUnitValueDate: string;
	    netEstimatedUnit?: number;
	    netEstimatedUnitTime: string;
	    netAccumulated?: number;
	    netGrowth1?: number;
	    netGrowth3?: number;
	    netGrowth6?: number;
	    netGrowth12?: number;
	    netGrowth36?: number;
	    netGrowth60?: number;
	    netGrowthYTD?: number;
	    netGrowthAll?: number;
	
	    static createFrom(source: any = {}) {
	        return new FundBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.type = source["type"];
	        this.establishment = source["establishment"];
	        this.scale = source["scale"];
	        this.company = source["company"];
	        this.manager = source["manager"];
	        this.rating = source["rating"];
	        this.trackingTarget = source["trackingTarget"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netUnitValueDate = source["netUnitValueDate"];
	        this.netEstimatedUnit = source["netEstimatedUnit"];
	        this.netEstimatedUnitTime = source["netEstimatedUnitTime"];
	        this.netAccumulated = source["netAccumulated"];
	        this.netGrowth1 = source["netGrowth1"];
	        this.netGrowth3 = source["netGrowth3"];
	        this.netGrowth6 = source["netGrowth6"];
	        this.netGrowth12 = source["netGrowth12"];
	        this.netGrowth36 = source["netGrowth36"];
	        this.netGrowth60 = source["netGrowth60"];
	        this.netGrowthYTD = source["netGrowthYTD"];
	        this.netGrowthAll = source["netGrowthAll"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FollowedFund {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    code: string;
	    name: string;
	    netUnitValue?: number;
	    netUnitValueDate: string;
	    netEstimatedUnit?: number;
	    netEstimatedUnitTime: string;
	    netAccumulated?: number;
	    netEstimatedRate?: number;
	    fundBasic: FundBasic;
	
	    static createFrom(source: any = {}) {
	        return new FollowedFund(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.netUnitValue = source["netUnitValue"];
	        this.netUnitValueDate = source["netUnitValueDate"];
	        this.netEstimatedUnit = source["netEstimatedUnit"];
	        this.netEstimatedUnitTime = source["netEstimatedUnitTime"];
	        this.netAccumulated = source["netAccumulated"];
	        this.netEstimatedRate = source["netEstimatedRate"];
	        this.fundBasic = this.convertValues(source["fundBasic"], FundBasic);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Group {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    name: string;
	    sort: number;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.name = source["name"];
	        this.sort = source["sort"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GroupStock {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    stockCode: string;
	    groupId: number;
	    groupInfo: Group;
	
	    static createFrom(source: any = {}) {
	        return new GroupStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.stockCode = source["stockCode"];
	        this.groupId = source["groupId"];
	        this.groupInfo = this.convertValues(source["groupInfo"], Group);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FollowedStock {
	    StockCode: string;
	    Name: string;
	    Volume: number;
	    CostPrice: number;
	    Price: number;
	    PriceChange: number;
	    ChangePercent: number;
	    AlarmChangePercent: number;
	    AlarmPrice: number;
	    // Go type: time
	    Time: any;
	    Sort: number;
	    Cron?: string;
	    IsDel: number;
	    Groups: GroupStock[];
	    AiConfigId: number;
	    EntryPrice: number;
	    TakeProfitPrice: number;
	    StopLossPrice: number;
	    FollowPrice: number;
	
	    static createFrom(source: any = {}) {
	        return new FollowedStock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.StockCode = source["StockCode"];
	        this.Name = source["Name"];
	        this.Volume = source["Volume"];
	        this.CostPrice = source["CostPrice"];
	        this.Price = source["Price"];
	        this.PriceChange = source["PriceChange"];
	        this.ChangePercent = source["ChangePercent"];
	        this.AlarmChangePercent = source["AlarmChangePercent"];
	        this.AlarmPrice = source["AlarmPrice"];
	        this.Time = this.convertValues(source["Time"], null);
	        this.Sort = source["Sort"];
	        this.Cron = source["Cron"];
	        this.IsDel = source["IsDel"];
	        this.Groups = this.convertValues(source["Groups"], GroupStock);
	        this.AiConfigId = source["AiConfigId"];
	        this.EntryPrice = source["EntryPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.FollowPrice = source["FollowPrice"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class PaperAccount {
	    id: number;
	    name: string;
	    cash: number;
	    initialCash: number;
	    equity: number;
	    valuationStatus: string;
	    unpricedPositions: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.cash = source["cash"];
	        this.initialCash = source["initialCash"];
	        this.equity = source["equity"];
	        this.valuationStatus = source["valuationStatus"];
	        this.unpricedPositions = source["unpricedPositions"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperEquityPoint {
	    id: number;
	    accountId: number;
	    dayKey: string;
	    equity: number;
	    cash: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperEquityPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.dayKey = source["dayKey"];
	        this.equity = source["equity"];
	        this.cash = source["cash"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperFill {
	    id: number;
	    accountId: number;
	    orderId: number;
	    stockCode: string;
	    stockName: string;
	    side: string;
	    price: number;
	    volume: number;
	    fee: number;
	    strategyTag: string;
	    // Go type: time
	    filledAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperFill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.orderId = source["orderId"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.side = source["side"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.fee = source["fee"];
	        this.strategyTag = source["strategyTag"];
	        this.filledAt = this.convertValues(source["filledAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperOrder {
	    id: number;
	    accountId: number;
	    stockCode: string;
	    stockName: string;
	    side: string;
	    status: string;
	    price: number;
	    volume: number;
	    filledPrice: number;
	    filledVol: number;
	    fee: number;
	    reason: string;
	    strategyTag: string;
	    rejectCode: string;
	    rejectReason: string;
	    fillAttemptCount: number;
	    execMode: string;
	    clientOrderId: string;
	    execBackend: string;
	    brokerOrderId: string;
	    externalOrderId: string;
	    brokerStatus: string;
	    // Go type: time
	    filledAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.side = source["side"];
	        this.status = source["status"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.filledPrice = source["filledPrice"];
	        this.filledVol = source["filledVol"];
	        this.fee = source["fee"];
	        this.reason = source["reason"];
	        this.strategyTag = source["strategyTag"];
	        this.rejectCode = source["rejectCode"];
	        this.rejectReason = source["rejectReason"];
	        this.fillAttemptCount = source["fillAttemptCount"];
	        this.execMode = source["execMode"];
	        this.clientOrderId = source["clientOrderId"];
	        this.execBackend = source["execBackend"];
	        this.brokerOrderId = source["brokerOrderId"];
	        this.externalOrderId = source["externalOrderId"];
	        this.brokerStatus = source["brokerStatus"];
	        this.filledAt = this.convertValues(source["filledAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperPosition {
	    id: number;
	    accountId: number;
	    stockCode: string;
	    stockName: string;
	    volume: number;
	    sellable: number;
	    avgCost: number;
	    markPrice: number;
	    // Go type: time
	    markedAt?: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperPosition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.volume = source["volume"];
	        this.sellable = source["sellable"];
	        this.avgCost = source["avgCost"];
	        this.markPrice = source["markPrice"];
	        this.markedAt = this.convertValues(source["markedAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperAccountSnapshot {
	    account: PaperAccount;
	    positions: PaperPosition[];
	    orders: PaperOrder[];
	    fills: PaperFill[];
	    equity: PaperEquityPoint[];
	
	    static createFrom(source: any = {}) {
	        return new PaperAccountSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account = this.convertValues(source["account"], PaperAccount);
	        this.positions = this.convertValues(source["positions"], PaperPosition);
	        this.orders = this.convertValues(source["orders"], PaperOrder);
	        this.fills = this.convertValues(source["fills"], PaperFill);
	        this.equity = this.convertValues(source["equity"], PaperEquityPoint);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperBorrowPool {
	    id: number;
	    stockCode: string;
	    stockName: string;
	    availableQuantity: number;
	    collateralRate: number;
	    financeMarginRatio: number;
	    securitiesMarginRatio: number;
	    enabled: boolean;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperBorrowPool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.availableQuantity = source["availableQuantity"];
	        this.collateralRate = source["collateralRate"];
	        this.financeMarginRatio = source["financeMarginRatio"];
	        this.securitiesMarginRatio = source["securitiesMarginRatio"];
	        this.enabled = source["enabled"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class PaperFinanceLiability {
	    id: number;
	    accountId: number;
	    stockCode: string;
	    principal: number;
	    accruedInterest: number;
	    quantity: number;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperFinanceLiability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.stockCode = source["stockCode"];
	        this.principal = source["principal"];
	        this.accruedInterest = source["accruedInterest"];
	        this.quantity = source["quantity"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperMarginAccount {
	    id: number;
	    accountId: number;
	    mode: string;
	    financeCreditLimit: number;
	    securitiesCreditLimit: number;
	    warningRatio: number;
	    closeoutRatio: number;
	    financeAnnualRate: number;
	    securitiesAnnualRate: number;
	    // Go type: time
	    lastAccruedAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperMarginAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.mode = source["mode"];
	        this.financeCreditLimit = source["financeCreditLimit"];
	        this.securitiesCreditLimit = source["securitiesCreditLimit"];
	        this.warningRatio = source["warningRatio"];
	        this.closeoutRatio = source["closeoutRatio"];
	        this.financeAnnualRate = source["financeAnnualRate"];
	        this.securitiesAnnualRate = source["securitiesAnnualRate"];
	        this.lastAccruedAt = this.convertValues(source["lastAccruedAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperMarginLedger {
	    id: number;
	    accountId: number;
	    orderId: number;
	    type: string;
	    stockCode: string;
	    cashDelta: number;
	    debtDelta: number;
	    quantity: number;
	    description: string;
	    // Go type: time
	    occurredAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperMarginLedger(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.orderId = source["orderId"];
	        this.type = source["type"];
	        this.stockCode = source["stockCode"];
	        this.cashDelta = source["cashDelta"];
	        this.debtDelta = source["debtDelta"];
	        this.quantity = source["quantity"];
	        this.description = source["description"];
	        this.occurredAt = this.convertValues(source["occurredAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperMarginOrder {
	    id: number;
	    accountId: number;
	    kind: string;
	    stockCode: string;
	    stockName: string;
	    status: string;
	    price: number;
	    volume: number;
	    fee: number;
	    reason: string;
	    reasonCode: string;
	    // Go type: time
	    filledAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperMarginOrder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.kind = source["kind"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.status = source["status"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.fee = source["fee"];
	        this.reason = source["reason"];
	        this.reasonCode = source["reasonCode"];
	        this.filledAt = this.convertValues(source["filledAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperMarginRiskEvent {
	    id: number;
	    accountId: number;
	    level: string;
	    reasonCode: string;
	    message: string;
	    maintenanceRatio: number;
	    resolved: boolean;
	    // Go type: time
	    occurredAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperMarginRiskEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.level = source["level"];
	        this.reasonCode = source["reasonCode"];
	        this.message = source["message"];
	        this.maintenanceRatio = source["maintenanceRatio"];
	        this.resolved = source["resolved"];
	        this.occurredAt = this.convertValues(source["occurredAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperOpenBuyConfig {
	    enablePaperOpenBuy: boolean;
	    paperOpenBuyCodes: string[];
	    openBuyAmountPerStock: number;
	    allowWhitelistFallback: boolean;
	    enableRiskFilter: boolean;
	    planMarketLevel: number;
	    blockNewEntriesOnDefense: boolean;
	    maxGrossExposurePct: number;
	    maxSingleNamePct: number;
	    maxDailyLossPct: number;
	    currentDailyPnlPct: number;
	
	    static createFrom(source: any = {}) {
	        return new PaperOpenBuyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enablePaperOpenBuy = source["enablePaperOpenBuy"];
	        this.paperOpenBuyCodes = source["paperOpenBuyCodes"];
	        this.openBuyAmountPerStock = source["openBuyAmountPerStock"];
	        this.allowWhitelistFallback = source["allowWhitelistFallback"];
	        this.enableRiskFilter = source["enableRiskFilter"];
	        this.planMarketLevel = source["planMarketLevel"];
	        this.blockNewEntriesOnDefense = source["blockNewEntriesOnDefense"];
	        this.maxGrossExposurePct = source["maxGrossExposurePct"];
	        this.maxSingleNamePct = source["maxSingleNamePct"];
	        this.maxDailyLossPct = source["maxDailyLossPct"];
	        this.currentDailyPnlPct = source["currentDailyPnlPct"];
	    }
	}
	export class PaperOpenBuyItemResult {
	    stockCode: string;
	    stockName: string;
	    price: number;
	    volume: number;
	    ok: boolean;
	    error?: string;
	    orderId?: number;
	    status?: string;
	
	    static createFrom(source: any = {}) {
	        return new PaperOpenBuyItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.ok = source["ok"];
	        this.error = source["error"];
	        this.orderId = source["orderId"];
	        this.status = source["status"];
	    }
	}
	export class PaperOpenBuyResult {
	    enabled: boolean;
	    planId?: number;
	    codes: string[];
	    items: PaperOpenBuyItemResult[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PaperOpenBuyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.planId = source["planId"];
	        this.codes = source["codes"];
	        this.items = this.convertValues(source["items"], PaperOpenBuyItemResult);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperOpenBuyStatus {
	    enablePaperOpenBuy: boolean;
	    tradeDate: string;
	    candidatePoolStatus: string;
	    candidateCount: number;
	    tradePlanStatus: string;
	    tradePlanCount: number;
	    executionReady: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PaperOpenBuyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enablePaperOpenBuy = source["enablePaperOpenBuy"];
	        this.tradeDate = source["tradeDate"];
	        this.candidatePoolStatus = source["candidatePoolStatus"];
	        this.candidateCount = source["candidateCount"];
	        this.tradePlanStatus = source["tradePlanStatus"];
	        this.tradePlanCount = source["tradePlanCount"];
	        this.executionReady = source["executionReady"];
	        this.message = source["message"];
	    }
	}
	
	export class PaperOrderHealthCounts {
	    submitted: number;
	    pending: number;
	    filled: number;
	    rejected: number;
	    cancelled: number;
	    processing: number;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrderHealthCounts(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.submitted = source["submitted"];
	        this.pending = source["pending"];
	        this.filled = source["filled"];
	        this.rejected = source["rejected"];
	        this.cancelled = source["cancelled"];
	        this.processing = source["processing"];
	    }
	}
	export class PaperOrderHealthRates {
	    fillRate: number;
	    rejectRate: number;
	    pendingRatio: number;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrderHealthRates(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fillRate = source["fillRate"];
	        this.rejectRate = source["rejectRate"];
	        this.pendingRatio = source["pendingRatio"];
	    }
	}
	export class PaperOrderLifecycleGap {
	    orderId: number;
	    symbol: string;
	    issue: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrderLifecycleGap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orderId = source["orderId"];
	        this.symbol = source["symbol"];
	        this.issue = source["issue"];
	        this.detail = source["detail"];
	    }
	}
	export class PaperOrderOrphan {
	    orderId: number;
	    symbol: string;
	    side: string;
	    price: number;
	    volume: number;
	    execMode: string;
	    age: number;
	    orphanKind: string;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrderOrphan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orderId = source["orderId"];
	        this.symbol = source["symbol"];
	        this.side = source["side"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.execMode = source["execMode"];
	        this.age = source["age"];
	        this.orphanKind = source["orphanKind"];
	        this.hint = source["hint"];
	    }
	}
	export class PaperOrderRejectStat {
	    code: string;
	    count: number;
	    percentage: number;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrderRejectStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.count = source["count"];
	        this.percentage = source["percentage"];
	    }
	}
	export class PaperOrderHealthReport {
	    tradeDate: string;
	    counts: PaperOrderHealthCounts;
	    rates: PaperOrderHealthRates;
	    rejectBreakdown: PaperOrderRejectStat[];
	    orphans: PaperOrderOrphan[];
	    lifecycleGaps: PaperOrderLifecycleGap[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new PaperOrderHealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tradeDate = source["tradeDate"];
	        this.counts = this.convertValues(source["counts"], PaperOrderHealthCounts);
	        this.rates = this.convertValues(source["rates"], PaperOrderHealthRates);
	        this.rejectBreakdown = this.convertValues(source["rejectBreakdown"], PaperOrderRejectStat);
	        this.orphans = this.convertValues(source["orphans"], PaperOrderOrphan);
	        this.lifecycleGaps = this.convertValues(source["lifecycleGaps"], PaperOrderLifecycleGap);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class PaperSecuritiesLiability {
	    id: number;
	    accountId: number;
	    stockCode: string;
	    stockName: string;
	    quantity: number;
	    avgPrice: number;
	    accruedFee: number;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new PaperSecuritiesLiability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.quantity = source["quantity"];
	        this.avgPrice = source["avgPrice"];
	        this.accruedFee = source["accruedFee"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PaperSubmitOrderReq {
	    accountId: number;
	    stockCode: string;
	    stockName: string;
	    side: string;
	    price: number;
	    volume: number;
	    reason: string;
	    strategyTag: string;
	    autoFill: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PaperSubmitOrderReq(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.side = source["side"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.reason = source["reason"];
	        this.strategyTag = source["strategyTag"];
	        this.autoFill = source["autoFill"];
	    }
	}
	export class SettingConfig {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    tushareToken: string;
	    localPushEnable: boolean;
	    dingPushEnable: boolean;
	    dingRobot: string;
	    updateBasicInfoOnStart: boolean;
	    refreshInterval: number;
	    openAiEnable: boolean;
	    prompt: string;
	    checkUpdate: boolean;
	    questionTemplate: string;
	    crawlTimeOut: number;
	    kDays: number;
	    enableDanmu: boolean;
	    browserPath: string;
	    enableNews: boolean;
	    darkTheme: boolean;
	    browserPoolSize: number;
	    enableFund: boolean;
	    enablePushNews: boolean;
	    enableOnlyPushRedNews: boolean;
	    sponsorCode: string;
	    httpProxy: string;
	    httpProxyEnabled: boolean;
	    enableAgent: boolean;
	    qgqpBId: string;
	    windowWidth: number;
	    windowHeight: number;
	    signalParams: string;
	    candidate_pool_score_threshold: number;
	    aiConfigs: AIConfig[];
	
	    static createFrom(source: any = {}) {
	        return new SettingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.tushareToken = source["tushareToken"];
	        this.localPushEnable = source["localPushEnable"];
	        this.dingPushEnable = source["dingPushEnable"];
	        this.dingRobot = source["dingRobot"];
	        this.updateBasicInfoOnStart = source["updateBasicInfoOnStart"];
	        this.refreshInterval = source["refreshInterval"];
	        this.openAiEnable = source["openAiEnable"];
	        this.prompt = source["prompt"];
	        this.checkUpdate = source["checkUpdate"];
	        this.questionTemplate = source["questionTemplate"];
	        this.crawlTimeOut = source["crawlTimeOut"];
	        this.kDays = source["kDays"];
	        this.enableDanmu = source["enableDanmu"];
	        this.browserPath = source["browserPath"];
	        this.enableNews = source["enableNews"];
	        this.darkTheme = source["darkTheme"];
	        this.browserPoolSize = source["browserPoolSize"];
	        this.enableFund = source["enableFund"];
	        this.enablePushNews = source["enablePushNews"];
	        this.enableOnlyPushRedNews = source["enableOnlyPushRedNews"];
	        this.sponsorCode = source["sponsorCode"];
	        this.httpProxy = source["httpProxy"];
	        this.httpProxyEnabled = source["httpProxyEnabled"];
	        this.enableAgent = source["enableAgent"];
	        this.qgqpBId = source["qgqpBId"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.signalParams = source["signalParams"];
	        this.candidate_pool_score_threshold = source["candidate_pool_score_threshold"];
	        this.aiConfigs = this.convertValues(source["aiConfigs"], AIConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SignalScanTaskView {
	    taskId: string;
	    status: string;
	    session: string;
	    strategyId: string;
	    strategyName: string;
	    startTime: string;
	    endTime?: string;
	    durationMs: number;
	    snapshotId?: number;
	    hitTotal: number;
	    scannedTotal: number;
	    message: string;
	    error?: string;
	    phase?: string;
	    done: number;
	    total: number;
	    tradeDate?: string;
	
	    static createFrom(source: any = {}) {
	        return new SignalScanTaskView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.status = source["status"];
	        this.session = source["session"];
	        this.strategyId = source["strategyId"];
	        this.strategyName = source["strategyName"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.durationMs = source["durationMs"];
	        this.snapshotId = source["snapshotId"];
	        this.hitTotal = source["hitTotal"];
	        this.scannedTotal = source["scannedTotal"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.phase = source["phase"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.tradeDate = source["tradeDate"];
	    }
	}
	export class StockBasic {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    ts_code: string;
	    symbol: string;
	    name: string;
	    area: string;
	    industry: string;
	    fullname: string;
	    enname: string;
	    cnspell: string;
	    market: string;
	    exchange: string;
	    curr_type: string;
	    list_status: string;
	    list_date: string;
	    delist_date: string;
	    is_hs: string;
	    act_name: string;
	    act_ent_type: string;
	    bk_name: string;
	    bk_code: string;
	
	    static createFrom(source: any = {}) {
	        return new StockBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.ts_code = source["ts_code"];
	        this.symbol = source["symbol"];
	        this.name = source["name"];
	        this.area = source["area"];
	        this.industry = source["industry"];
	        this.fullname = source["fullname"];
	        this.enname = source["enname"];
	        this.cnspell = source["cnspell"];
	        this.market = source["market"];
	        this.exchange = source["exchange"];
	        this.curr_type = source["curr_type"];
	        this.list_status = source["list_status"];
	        this.list_date = source["list_date"];
	        this.delist_date = source["delist_date"];
	        this.is_hs = source["is_hs"];
	        this.act_name = source["act_name"];
	        this.act_ent_type = source["act_ent_type"];
	        this.bk_name = source["bk_name"];
	        this.bk_code = source["bk_code"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockChangeItem {
	    time: string;
	    code: string;
	    name: string;
	    market: number;
	    changeType: number;
	    typeName: string;
	    volume: number;
	    price: number;
	    changeRate: number;
	    amount: number;
	    industry: string;
	    concept: string;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.market = source["market"];
	        this.changeType = source["changeType"];
	        this.typeName = source["typeName"];
	        this.volume = source["volume"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.amount = source["amount"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	    }
	}
	export class StockChangesResponse {
	    totalCount: number;
	    data: StockChangeItem[];
	
	    static createFrom(source: any = {}) {
	        return new StockChangesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCount = source["totalCount"];
	        this.data = this.convertValues(source["data"], StockChangeItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    "日期": string;
	    "时间": string;
	    "股票代码": string;
	    "股票名称": string;
	    "上次当前价格": number;
	    "当前价格": string;
	    "成交的股票数": string;
	    "成交金额": string;
	    "今日开盘价": string;
	    "昨日收盘价": string;
	    "今日最高价": string;
	    "今日最低价": string;
	    "竞买价": string;
	    "竞卖价": string;
	    "买一报价": string;
	    "买一申报": string;
	    "买二报价": string;
	    "买二申报": string;
	    "买三报价": string;
	    "买三申报": string;
	    "买四报价": string;
	    "买四申报": string;
	    "买五报价": string;
	    "买五申报": string;
	    "卖一报价": string;
	    "卖一申报": string;
	    "卖二报价": string;
	    "卖二申报": string;
	    "卖三报价": string;
	    "卖三申报": string;
	    "卖四报价": string;
	    "卖四申报": string;
	    "卖五报价": string;
	    "卖五申报": string;
	    "市场": string;
	    "盘前盘后": string;
	    "盘前盘后涨跌幅": string;
	    changePercent: number;
	    changePrice: number;
	    highRate: number;
	    lowRate: number;
	    costPrice: number;
	    costVolume: number;
	    profit: number;
	    profitAmount: number;
	    profitAmountToday: number;
	    "关注价格": number;
	    followProfit: number;
	    sort: number;
	    alarmChangePercent: number;
	    alarmPrice: number;
	    "关注时间": string;
	    "所属行业": string;
	    "所属板块": string;
	    Groups: GroupStock[];
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this["日期"] = source["日期"];
	        this["时间"] = source["时间"];
	        this["股票代码"] = source["股票代码"];
	        this["股票名称"] = source["股票名称"];
	        this["上次当前价格"] = source["上次当前价格"];
	        this["当前价格"] = source["当前价格"];
	        this["成交的股票数"] = source["成交的股票数"];
	        this["成交金额"] = source["成交金额"];
	        this["今日开盘价"] = source["今日开盘价"];
	        this["昨日收盘价"] = source["昨日收盘价"];
	        this["今日最高价"] = source["今日最高价"];
	        this["今日最低价"] = source["今日最低价"];
	        this["竞买价"] = source["竞买价"];
	        this["竞卖价"] = source["竞卖价"];
	        this["买一报价"] = source["买一报价"];
	        this["买一申报"] = source["买一申报"];
	        this["买二报价"] = source["买二报价"];
	        this["买二申报"] = source["买二申报"];
	        this["买三报价"] = source["买三报价"];
	        this["买三申报"] = source["买三申报"];
	        this["买四报价"] = source["买四报价"];
	        this["买四申报"] = source["买四申报"];
	        this["买五报价"] = source["买五报价"];
	        this["买五申报"] = source["买五申报"];
	        this["卖一报价"] = source["卖一报价"];
	        this["卖一申报"] = source["卖一申报"];
	        this["卖二报价"] = source["卖二报价"];
	        this["卖二申报"] = source["卖二申报"];
	        this["卖三报价"] = source["卖三报价"];
	        this["卖三申报"] = source["卖三申报"];
	        this["卖四报价"] = source["卖四报价"];
	        this["卖四申报"] = source["卖四申报"];
	        this["卖五报价"] = source["卖五报价"];
	        this["卖五申报"] = source["卖五申报"];
	        this["市场"] = source["市场"];
	        this["盘前盘后"] = source["盘前盘后"];
	        this["盘前盘后涨跌幅"] = source["盘前盘后涨跌幅"];
	        this.changePercent = source["changePercent"];
	        this.changePrice = source["changePrice"];
	        this.highRate = source["highRate"];
	        this.lowRate = source["lowRate"];
	        this.costPrice = source["costPrice"];
	        this.costVolume = source["costVolume"];
	        this.profit = source["profit"];
	        this.profitAmount = source["profitAmount"];
	        this.profitAmountToday = source["profitAmountToday"];
	        this["关注价格"] = source["关注价格"];
	        this.followProfit = source["followProfit"];
	        this.sort = source["sort"];
	        this.alarmChangePercent = source["alarmChangePercent"];
	        this.alarmPrice = source["alarmPrice"];
	        this["关注时间"] = source["关注时间"];
	        this["所属行业"] = source["所属行业"];
	        this["所属板块"] = source["所属板块"];
	        this.Groups = this.convertValues(source["Groups"], GroupStock);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StrategyPerformanceRow {
	    strategyName: string;
	    strategyVersion: string;
	    signalTag: string;
	    candidateCount: number;
	    planCount: number;
	    executedCount: number;
	    skippedCount: number;
	    winCount: number;
	    lossCount: number;
	    pnl: number;
	
	    static createFrom(source: any = {}) {
	        return new StrategyPerformanceRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategyName = source["strategyName"];
	        this.strategyVersion = source["strategyVersion"];
	        this.signalTag = source["signalTag"];
	        this.candidateCount = source["candidateCount"];
	        this.planCount = source["planCount"];
	        this.executedCount = source["executedCount"];
	        this.skippedCount = source["skippedCount"];
	        this.winCount = source["winCount"];
	        this.lossCount = source["lossCount"];
	        this.pnl = source["pnl"];
	    }
	}
	export class TradePlanAnalysis {
	    tradeDate: string;
	    pool?: AnalysisPoolInfo;
	    plan?: AnalysisPlanInfo;
	    items: AnalysisChainItem[];
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new TradePlanAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tradeDate = source["tradeDate"];
	        this.pool = this.convertValues(source["pool"], AnalysisPoolInfo);
	        this.plan = this.convertValues(source["plan"], AnalysisPlanInfo);
	        this.items = this.convertValues(source["items"], AnalysisChainItem);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecord {
	    ID: number;
	    StockCode: string;
	    StockName: string;
	    Direction: string;
	    Status: string;
	    Price: number;
	    Volume: number;
	    Amount: number;
	    // Go type: time
	    TradingTime: any;
	    Reason: string;
	    StopLossPrice: number;
	    TakeProfitPrice: number;
	    Fee: number;
	    MarketValue: number;
	    Mindset: string;
	    recordedClosePrice: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StockCode = source["StockCode"];
	        this.StockName = source["StockName"];
	        this.Direction = source["Direction"];
	        this.Status = source["Status"];
	        this.Price = source["Price"];
	        this.Volume = source["Volume"];
	        this.Amount = source["Amount"];
	        this.TradingTime = this.convertValues(source["TradingTime"], null);
	        this.Reason = source["Reason"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.Fee = source["Fee"];
	        this.MarketValue = source["MarketValue"];
	        this.Mindset = source["Mindset"];
	        this.recordedClosePrice = source["recordedClosePrice"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecordItem {
	    ID: number;
	    StockCode: string;
	    StockName: string;
	    Direction: string;
	    Status: string;
	    Price: number;
	    Volume: number;
	    Amount: number;
	    // Go type: time
	    TradingTime: any;
	    Reason: string;
	    StopLossPrice: number;
	    TakeProfitPrice: number;
	    Fee: number;
	    MarketValue: number;
	    Mindset: string;
	    recordedClosePrice: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    closePrice: number;
	    profitAmount: number;
	    profitPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.StockCode = source["StockCode"];
	        this.StockName = source["StockName"];
	        this.Direction = source["Direction"];
	        this.Status = source["Status"];
	        this.Price = source["Price"];
	        this.Volume = source["Volume"];
	        this.Amount = source["Amount"];
	        this.TradingTime = this.convertValues(source["TradingTime"], null);
	        this.Reason = source["Reason"];
	        this.StopLossPrice = source["StopLossPrice"];
	        this.TakeProfitPrice = source["TakeProfitPrice"];
	        this.Fee = source["Fee"];
	        this.MarketValue = source["MarketValue"];
	        this.Mindset = source["Mindset"];
	        this.recordedClosePrice = source["recordedClosePrice"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.closePrice = source["closePrice"];
	        this.profitAmount = source["profitAmount"];
	        this.profitPercent = source["profitPercent"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecordListQuery {
	    page: number;
	    pageSize: number;
	    keyword: string;
	    direction: string;
	    status: string;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordListQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.keyword = source["keyword"];
	        this.direction = source["direction"];
	        this.status = source["status"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class TradingRecordPageData {
	    list: TradingRecordItem[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], TradingRecordItem);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradingRecordStatistics {
	    totalBuyAmount: number;
	    totalSellAmount: number;
	    totalProfit: number;
	    profitRate: number;
	    holdingsAmount: number;
	    currentValue: number;
	    stockCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRecordStatistics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalBuyAmount = source["totalBuyAmount"];
	        this.totalSellAmount = source["totalSellAmount"];
	        this.totalProfit = source["totalProfit"];
	        this.profitRate = source["profitRate"];
	        this.holdingsAmount = source["holdingsAmount"];
	        this.currentValue = source["currentValue"];
	        this.stockCount = source["stockCount"];
	    }
	}

}

export namespace diagnostic {
	
	export class BuildBlock {
	    version_info: version.VersionInfo;
	    goos: string;
	    goarch: string;
	    build_mode: string;
	    identity_unknown: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BuildBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version_info = this.convertValues(source["version_info"], version.VersionInfo);
	        this.goos = source["goos"];
	        this.goarch = source["goarch"];
	        this.build_mode = source["build_mode"];
	        this.identity_unknown = source["identity_unknown"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DatabaseStatus {
	    ok: boolean;
	    integrity?: string;
	    trading_allowed: boolean;
	    backup_file_name?: string;
	    error_code?: string;
	    message_safe?: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.integrity = source["integrity"];
	        this.trading_allowed = source["trading_allowed"];
	        this.backup_file_name = source["backup_file_name"];
	        this.error_code = source["error_code"];
	        this.message_safe = source["message_safe"];
	    }
	}
	export class Environment {
	    os: string;
	    arch: string;
	    goos_goarch: string;
	    app_cwd_hash8?: string;
	
	    static createFrom(source: any = {}) {
	        return new Environment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.goos_goarch = source["goos_goarch"];
	        this.app_cwd_hash8 = source["app_cwd_hash8"];
	    }
	}
	export class ErrorEntry {
	    at: string;
	    source: string;
	    category: string;
	    code?: string;
	    message_safe: string;
	    page?: string;
	
	    static createFrom(source: any = {}) {
	        return new ErrorEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = source["at"];
	        this.source = source["source"];
	        this.category = source["category"];
	        this.code = source["code"];
	        this.message_safe = source["message_safe"];
	        this.page = source["page"];
	    }
	}
	export class ErrorSummary {
	    total_count: number;
	    primary_category?: string;
	    primary_code?: string;
	    message_safe?: string;
	    items?: ErrorEntry[];
	
	    static createFrom(source: any = {}) {
	        return new ErrorSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_count = source["total_count"];
	        this.primary_category = source["primary_category"];
	        this.primary_code = source["primary_code"];
	        this.message_safe = source["message_safe"];
	        this.items = this.convertValues(source["items"], ErrorEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LogSummary {
	    info_tail?: string[];
	    error_tail?: string[];
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new LogSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.info_tail = source["info_tail"];
	        this.error_tail = source["error_tail"];
	        this.note = source["note"];
	    }
	}
	export class ProviderModeSnapshot {
	    adoption: string;
	    kill_switch: boolean;
	    account_whitelist_count: number;
	    strategy_whitelist_count: number;
	    date_whitelist_count: number;
	    last_plan_provider_mode?: string;
	    last_plan_decision_provider?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderModeSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adoption = source["adoption"];
	        this.kill_switch = source["kill_switch"];
	        this.account_whitelist_count = source["account_whitelist_count"];
	        this.strategy_whitelist_count = source["strategy_whitelist_count"];
	        this.date_whitelist_count = source["date_whitelist_count"];
	        this.last_plan_provider_mode = source["last_plan_provider_mode"];
	        this.last_plan_decision_provider = source["last_plan_decision_provider"];
	    }
	}
	export class TradingPaperSummary {
	    has_account: boolean;
	    order_count: number;
	    fill_count: number;
	    filled_order_count: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingPaperSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_account = source["has_account"];
	        this.order_count = source["order_count"];
	        this.fill_count = source["fill_count"];
	        this.filled_order_count = source["filled_order_count"];
	    }
	}
	export class TradingExecutionSummary {
	    phase: string;
	    ready: boolean;
	    executor_configured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TradingExecutionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.ready = source["ready"];
	        this.executor_configured = source["executor_configured"];
	    }
	}
	export class TradingRiskSummary {
	    status: string;
	    accepted_count: number;
	    filtered_count: number;
	    market_level: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingRiskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.accepted_count = source["accepted_count"];
	        this.filtered_count = source["filtered_count"];
	        this.market_level = source["market_level"];
	    }
	}
	export class TradingPlanSummary {
	    status: string;
	    item_count: number;
	    pending_count: number;
	    filled_count: number;
	    skipped_count: number;
	    error_count: number;
	    reconcile_recommended: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TradingPlanSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.item_count = source["item_count"];
	        this.pending_count = source["pending_count"];
	        this.filled_count = source["filled_count"];
	        this.skipped_count = source["skipped_count"];
	        this.error_count = source["error_count"];
	        this.reconcile_recommended = source["reconcile_recommended"];
	    }
	}
	export class TradingCandidateSummary {
	    status: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new TradingCandidateSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.count = source["count"];
	    }
	}
	export class TradingStatusSummary {
	    trade_date: string;
	    is_weekday: boolean;
	    paper_open_buy_enabled: boolean;
	    candidate: TradingCandidateSummary;
	    plan: TradingPlanSummary;
	    risk: TradingRiskSummary;
	    execution: TradingExecutionSummary;
	    paper: TradingPaperSummary;
	    block_reasons?: string[];
	    message_safe?: string;
	
	    static createFrom(source: any = {}) {
	        return new TradingStatusSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trade_date = source["trade_date"];
	        this.is_weekday = source["is_weekday"];
	        this.paper_open_buy_enabled = source["paper_open_buy_enabled"];
	        this.candidate = this.convertValues(source["candidate"], TradingCandidateSummary);
	        this.plan = this.convertValues(source["plan"], TradingPlanSummary);
	        this.risk = this.convertValues(source["risk"], TradingRiskSummary);
	        this.execution = this.convertValues(source["execution"], TradingExecutionSummary);
	        this.paper = this.convertValues(source["paper"], TradingPaperSummary);
	        this.block_reasons = source["block_reasons"];
	        this.message_safe = source["message_safe"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class JobStatusEntry {
	    job_name: string;
	    last_run?: string;
	    last_status?: string;
	    last_error_safe?: string;
	    expected_schedule?: string;
	    catch_up_policy?: string;
	
	    static createFrom(source: any = {}) {
	        return new JobStatusEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_name = source["job_name"];
	        this.last_run = source["last_run"];
	        this.last_status = source["last_status"];
	        this.last_error_safe = source["last_error_safe"];
	        this.expected_schedule = source["expected_schedule"];
	        this.catch_up_policy = source["catch_up_policy"];
	    }
	}
	export class MigrationSnapshot {
	    current_version: number;
	    required_version: number;
	    status: string;
	    missing_tables?: string[];
	    missing_columns?: string[];
	    missing_indexes?: string[];
	    trading_allowed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MigrationSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_version = source["current_version"];
	        this.required_version = source["required_version"];
	        this.status = source["status"];
	        this.missing_tables = source["missing_tables"];
	        this.missing_columns = source["missing_columns"];
	        this.missing_indexes = source["missing_indexes"];
	        this.trading_allowed = source["trading_allowed"];
	    }
	}
	export class Info {
	    schema_version: string;
	    diagnostic_id: string;
	    timestamp: string;
	    version: string;
	    build_time: string;
	    git_commit: string;
	    commit_hash: string;
	    channel: string;
	    build_mode: string;
	    version_info: version.VersionInfo;
	    build: BuildBlock;
	    migration: MigrationSnapshot;
	    environment: Environment;
	    database: DatabaseStatus;
	    last_jobs?: JobStatusEntry[];
	    trading_status: TradingStatusSummary;
	    provider_mode: ProviderModeSnapshot;
	    errors?: ErrorEntry[];
	    error_summary: ErrorSummary;
	    log_summary: LogSummary;
	    trading_allowed?: boolean;
	    crash_reports_present: boolean;
	    crash_report_names?: string[];
	    error_category: string;
	    error_code?: string;
	    message_safe: string;
	    surface?: string;
	    export_note: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.diagnostic_id = source["diagnostic_id"];
	        this.timestamp = source["timestamp"];
	        this.version = source["version"];
	        this.build_time = source["build_time"];
	        this.git_commit = source["git_commit"];
	        this.commit_hash = source["commit_hash"];
	        this.channel = source["channel"];
	        this.build_mode = source["build_mode"];
	        this.version_info = this.convertValues(source["version_info"], version.VersionInfo);
	        this.build = this.convertValues(source["build"], BuildBlock);
	        this.migration = this.convertValues(source["migration"], MigrationSnapshot);
	        this.environment = this.convertValues(source["environment"], Environment);
	        this.database = this.convertValues(source["database"], DatabaseStatus);
	        this.last_jobs = this.convertValues(source["last_jobs"], JobStatusEntry);
	        this.trading_status = this.convertValues(source["trading_status"], TradingStatusSummary);
	        this.provider_mode = this.convertValues(source["provider_mode"], ProviderModeSnapshot);
	        this.errors = this.convertValues(source["errors"], ErrorEntry);
	        this.error_summary = this.convertValues(source["error_summary"], ErrorSummary);
	        this.log_summary = this.convertValues(source["log_summary"], LogSummary);
	        this.trading_allowed = source["trading_allowed"];
	        this.crash_reports_present = source["crash_reports_present"];
	        this.crash_report_names = source["crash_report_names"];
	        this.error_category = source["error_category"];
	        this.error_code = source["error_code"];
	        this.message_safe = source["message_safe"];
	        this.surface = source["surface"];
	        this.export_note = source["export_note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	
	

}

export namespace execution {
	
	export class AccountConfig {
	    accountId: number;
	    mode: string;
	    financeCreditLimit: number;
	    securitiesCreditLimit: number;
	    warningRatio: number;
	    closeoutRatio: number;
	    financeAnnualRate: number;
	    securitiesAnnualRate: number;
	
	    static createFrom(source: any = {}) {
	        return new AccountConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.mode = source["mode"];
	        this.financeCreditLimit = source["financeCreditLimit"];
	        this.securitiesCreditLimit = source["securitiesCreditLimit"];
	        this.warningRatio = source["warningRatio"];
	        this.closeoutRatio = source["closeoutRatio"];
	        this.financeAnnualRate = source["financeAnnualRate"];
	        this.securitiesAnnualRate = source["securitiesAnnualRate"];
	    }
	}
	export class AccrualResult {
	    accountId: number;
	    days: number;
	    financeInterest: number;
	    securitiesFee: number;
	    // Go type: time
	    accruedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new AccrualResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.days = source["days"];
	        this.financeInterest = source["financeInterest"];
	        this.securitiesFee = source["securitiesFee"];
	        this.accruedAt = this.convertValues(source["accruedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PositionMark {
	    stockCode: string;
	    price: number;
	
	    static createFrom(source: any = {}) {
	        return new PositionMark(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.price = source["price"];
	    }
	}
	export class Snapshot {
	    account: data.PaperAccount;
	    marginAccount: data.PaperMarginAccount;
	    positions: data.PaperPosition[];
	    financeLiabilities: data.PaperFinanceLiability[];
	    securitiesLiabilities: data.PaperSecuritiesLiability[];
	    orders: data.PaperMarginOrder[];
	    ledger: data.PaperMarginLedger[];
	    riskEvents: data.PaperMarginRiskEvent[];
	    metrics: risk.Metrics;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.account = this.convertValues(source["account"], data.PaperAccount);
	        this.marginAccount = this.convertValues(source["marginAccount"], data.PaperMarginAccount);
	        this.positions = this.convertValues(source["positions"], data.PaperPosition);
	        this.financeLiabilities = this.convertValues(source["financeLiabilities"], data.PaperFinanceLiability);
	        this.securitiesLiabilities = this.convertValues(source["securitiesLiabilities"], data.PaperSecuritiesLiability);
	        this.orders = this.convertValues(source["orders"], data.PaperMarginOrder);
	        this.ledger = this.convertValues(source["ledger"], data.PaperMarginLedger);
	        this.riskEvents = this.convertValues(source["riskEvents"], data.PaperMarginRiskEvent);
	        this.metrics = this.convertValues(source["metrics"], risk.Metrics);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SubmitRequest {
	    accountId: number;
	    kind: string;
	    stockCode: string;
	    stockName: string;
	    price: number;
	    volume: number;
	    reason: string;
	    marketLevel: number;
	    blockNewEntries: boolean;
	    maxExposurePct: number;
	    maxSingleNamePct: number;
	    maxGrossExposurePct: number;
	    maxDailyLossPct: number;
	    currentDailyPnlPct: number;
	
	    static createFrom(source: any = {}) {
	        return new SubmitRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.kind = source["kind"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.price = source["price"];
	        this.volume = source["volume"];
	        this.reason = source["reason"];
	        this.marketLevel = source["marketLevel"];
	        this.blockNewEntries = source["blockNewEntries"];
	        this.maxExposurePct = source["maxExposurePct"];
	        this.maxSingleNamePct = source["maxSingleNamePct"];
	        this.maxGrossExposurePct = source["maxGrossExposurePct"];
	        this.maxDailyLossPct = source["maxDailyLossPct"];
	        this.currentDailyPnlPct = source["currentDailyPnlPct"];
	    }
	}

}

export namespace job {
	
	export class ExecutionRecord {
	    id: string;
	    job: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    finished_at: any;
	    status: string;
	    error?: string;
	    trigger?: string;
	    // Go type: time
	    expected_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.job = source["job"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.finished_at = this.convertValues(source["finished_at"], null);
	        this.status = source["status"];
	        this.error = source["error"];
	        this.trigger = source["trigger"];
	        this.expected_at = this.convertValues(source["expected_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MissedInfo {
	    job: string;
	    expectedSchedule: string;
	    // Go type: time
	    expectedAt: any;
	    // Go type: time
	    detectedAt: any;
	    // Go type: time
	    lastRun?: any;
	    lastStatus?: string;
	    message: string;
	    allowManualOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MissedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job = source["job"];
	        this.expectedSchedule = source["expectedSchedule"];
	        this.expectedAt = this.convertValues(source["expectedAt"], null);
	        this.detectedAt = this.convertValues(source["detectedAt"], null);
	        this.lastRun = this.convertValues(source["lastRun"], null);
	        this.lastStatus = source["lastStatus"];
	        this.message = source["message"];
	        this.allowManualOnly = source["allowManualOnly"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RuntimeState {
	    jobName: string;
	    expectedSchedule: string;
	    // Go type: time
	    lastRun?: any;
	    lastStatus?: string;
	    lastError?: string;
	    // Go type: time
	    registeredAt: any;
	    catchUpPolicy: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jobName = source["jobName"];
	        this.expectedSchedule = source["expectedSchedule"];
	        this.lastRun = this.convertValues(source["lastRun"], null);
	        this.lastStatus = source["lastStatus"];
	        this.lastError = source["lastError"];
	        this.registeredAt = this.convertValues(source["registeredAt"], null);
	        this.catchUpPolicy = source["catchUpPolicy"];
	        this.description = source["description"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace lo {
	
	export class Tuple2_string_string_ {
	    A: string;
	    B: string;
	
	    static createFrom(source: any = {}) {
	        return new Tuple2_string_string_(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.A = source["A"];
	        this.B = source["B"];
	    }
	}

}

export namespace main {
	
	export class FeatureGateDecisionDTO {
	    allowed: boolean;
	    feature: string;
	    tier: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new FeatureGateDecisionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed = source["allowed"];
	        this.feature = source["feature"];
	        this.tier = source["tier"];
	        this.reason = source["reason"];
	    }
	}
	export class LoadingProgress {
	    percent: number;
	    message: string;
	    done: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LoadingProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.percent = source["percent"];
	        this.message = source["message"];
	        this.done = source["done"];
	    }
	}

}

export namespace marketstate {
	
	export class Snapshot {
	    state: string;
	    isTradingDay: boolean;
	    // Go type: time
	    localTime: any;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.isTradingDay = source["isTradingDay"];
	        this.localTime = this.convertValues(source["localTime"], null);
	        this.reason = source["reason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace models {
	
	export class AIResponseResult {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    chatId: string;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    question: string;
	    content: string;
	    IsDel: number;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.chatId = source["chatId"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.question = source["question"];
	        this.content = source["content"];
	        this.IsDel = source["IsDel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIResponseResultPageData {
	    list: AIResponseResult[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResultPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AIResponseResult);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AIResponseResultQuery {
	    page: number;
	    pageSize: number;
	    chatId: string;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    question: string;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new AIResponseResultQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.chatId = source["chatId"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.question = source["question"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class AiAssistantMessage {
	    role: string;
	    content: string;
	    reasoning: string;
	    time: string;
	    modelName?: string;
	    toolCalls?: number[];
	    toolResults?: number[];
	    timeline?: number[];
	
	    static createFrom(source: any = {}) {
	        return new AiAssistantMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.reasoning = source["reasoning"];
	        this.time = source["time"];
	        this.modelName = source["modelName"];
	        this.toolCalls = source["toolCalls"];
	        this.toolResults = source["toolResults"];
	        this.timeline = source["timeline"];
	    }
	}
	export class AiAssistantSessionResp {
	    messages: AiAssistantMessage[];
	    sessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new AiAssistantSessionResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], AiAssistantMessage);
	        this.sessionId = source["sessionId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiRecommendStocks {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    // Go type: time
	    dataTime?: any;
	    modelName: string;
	    rating: string;
	    stockCode: string;
	    stockName: string;
	    bkCode: string;
	    bkName: string;
	    stockPrice: string;
	    stockCurrentPrice: string;
	    stockCurrentPriceTime: string;
	    stockClosePrice: string;
	    stockPrePrice: string;
	    recommendReason: string;
	    recommendBuyPrice: string;
	    recommendBuyPriceMin: number;
	    recommendBuyPriceMax: number;
	    recommendStopProfitPrice: string;
	    recommendStopProfitPriceMin: number;
	    recommendStopProfitPriceMax: number;
	    recommendStopLossPrice: string;
	    riskRemarks: string;
	    remarks: string;
	    enableAlert: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.dataTime = this.convertValues(source["dataTime"], null);
	        this.modelName = source["modelName"];
	        this.rating = source["rating"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.bkCode = source["bkCode"];
	        this.bkName = source["bkName"];
	        this.stockPrice = source["stockPrice"];
	        this.stockCurrentPrice = source["stockCurrentPrice"];
	        this.stockCurrentPriceTime = source["stockCurrentPriceTime"];
	        this.stockClosePrice = source["stockClosePrice"];
	        this.stockPrePrice = source["stockPrePrice"];
	        this.recommendReason = source["recommendReason"];
	        this.recommendBuyPrice = source["recommendBuyPrice"];
	        this.recommendBuyPriceMin = source["recommendBuyPriceMin"];
	        this.recommendBuyPriceMax = source["recommendBuyPriceMax"];
	        this.recommendStopProfitPrice = source["recommendStopProfitPrice"];
	        this.recommendStopProfitPriceMin = source["recommendStopProfitPriceMin"];
	        this.recommendStopProfitPriceMax = source["recommendStopProfitPriceMax"];
	        this.recommendStopLossPrice = source["recommendStopLossPrice"];
	        this.riskRemarks = source["riskRemarks"];
	        this.remarks = source["remarks"];
	        this.enableAlert = source["enableAlert"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiRecommendStocksPageData {
	    list: AiRecommendStocks[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocksPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], AiRecommendStocks);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AiRecommendStocksQuery {
	    page: number;
	    pageSize: number;
	    modelName: string;
	    stockCode: string;
	    stockName: string;
	    bkCode: string;
	    bkName: string;
	    startDate: string;
	    endDate: string;
	    enableAlert?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiRecommendStocksQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.modelName = source["modelName"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.bkCode = source["bkCode"];
	        this.bkName = source["bkName"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.enableAlert = source["enableAlert"];
	    }
	}
	export class AllStockInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE: string;
	    CHANGE_RATE: string;
	    VOLUME_RATIO: string;
	    HIGH_PRICE: string;
	    LOW_PRICE: string;
	    PRE_CLOSE_PRICE: string;
	    VOLUME: string;
	    DEAL_AMOUNT: string;
	    TURNOVERRATE: string;
	    MARKET: string;
	    CONCEPT: string;
	    INDUSTRY: string;
	    MAX_TRADE_DATE: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.MARKET = source["MARKET"];
	        this.CONCEPT = source["CONCEPT"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.MAX_TRADE_DATE = source["MAX_TRADE_DATE"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AllStocksResp {
	    version: any;
	    // Go type: struct { Nextpage bool "json:\"nextpage\""; Currentpage int "json:\"currentpage\""; Data []models
	    result: any;
	    success: boolean;
	    message: string;
	    code: number;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new AllStocksResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.result = this.convertValues(source["result"], Object);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.code = source["code"];
	        this.url = source["url"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CandidatePoolItem {
	    id: number;
	    poolId: number;
	    tradeDate: string;
	    stockCode: string;
	    stockName: string;
	    rank: number;
	    score: number;
	    reason: string;
	    strategyName: string;
	    strategyVersion: string;
	    industry: string;
	    tagsJson: string;
	    signalTag: string;
	    signalScore: number;
	    signalSnapshotId: number;
	    decisionId: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CandidatePoolItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.poolId = source["poolId"];
	        this.tradeDate = source["tradeDate"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.rank = source["rank"];
	        this.score = source["score"];
	        this.reason = source["reason"];
	        this.strategyName = source["strategyName"];
	        this.strategyVersion = source["strategyVersion"];
	        this.industry = source["industry"];
	        this.tagsJson = source["tagsJson"];
	        this.signalTag = source["signalTag"];
	        this.signalScore = source["signalScore"];
	        this.signalSnapshotId = source["signalSnapshotId"];
	        this.decisionId = source["decisionId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CandidatePool {
	    id: number;
	    tradeDate: string;
	    // Go type: time
	    generatedAt: any;
	    source: string;
	    sourceRef: string;
	    status: string;
	    itemCount: number;
	    message: string;
	    configJson: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    items?: CandidatePoolItem[];
	
	    static createFrom(source: any = {}) {
	        return new CandidatePool(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.tradeDate = source["tradeDate"];
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.source = source["source"];
	        this.sourceRef = source["sourceRef"];
	        this.status = source["status"];
	        this.itemCount = source["itemCount"];
	        this.message = source["message"];
	        this.configJson = source["configJson"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.items = this.convertValues(source["items"], CandidatePoolItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class CronTask {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    cronExpr: string;
	    taskType: string;
	    target: string;
	    params: string;
	    enable: boolean;
	    // Go type: time
	    lastRunAt?: any;
	    // Go type: time
	    nextRunAt?: any;
	    runCount: number;
	    status: string;
	    description: string;
	    lastRunResult: string;
	
	    static createFrom(source: any = {}) {
	        return new CronTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.cronExpr = source["cronExpr"];
	        this.taskType = source["taskType"];
	        this.target = source["target"];
	        this.params = source["params"];
	        this.enable = source["enable"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.nextRunAt = this.convertValues(source["nextRunAt"], null);
	        this.runCount = source["runCount"];
	        this.status = source["status"];
	        this.description = source["description"];
	        this.lastRunResult = source["lastRunResult"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CronTaskPageResp {
	    total: number;
	    data: CronTask[];
	
	    static createFrom(source: any = {}) {
	        return new CronTaskPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], CronTask);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CronTaskQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    taskType: string;
	    status: string;
	    enable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CronTaskQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.taskType = source["taskType"];
	        this.status = source["status"];
	        this.enable = source["enable"];
	    }
	}
	export class Prompt {
	    ID: number;
	    name: string;
	    content: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new Prompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.type = source["type"];
	    }
	}
	export class PromptTemplate {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    name: string;
	    content: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.name = source["name"];
	        this.content = source["content"];
	        this.type = source["type"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PromptTemplatePageData {
	    list: PromptTemplate[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplatePageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], PromptTemplate);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PromptTemplateQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    type: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new PromptTemplateQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.content = source["content"];
	    }
	}
	export class ResearchSnapshotCandidate {
	    stockCode: string;
	    stockName: string;
	    signalScore: number;
	    signalTag: string;
	    direction: string;
	    statusText: string;
	    price: string;
	    industry: string;
	
	    static createFrom(source: any = {}) {
	        return new ResearchSnapshotCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.signalScore = source["signalScore"];
	        this.signalTag = source["signalTag"];
	        this.direction = source["direction"];
	        this.statusText = source["statusText"];
	        this.price = source["price"];
	        this.industry = source["industry"];
	    }
	}
	export class ResearchSnapshotCandidateList {
	    snapshotId: number;
	    snapshotTime: string;
	    tradeDate: string;
	    session: string;
	    strategyName: string;
	    minScore: number;
	    hitTotal: number;
	    itemCount: number;
	    message: string;
	    items: ResearchSnapshotCandidate[];
	
	    static createFrom(source: any = {}) {
	        return new ResearchSnapshotCandidateList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshotId = source["snapshotId"];
	        this.snapshotTime = source["snapshotTime"];
	        this.tradeDate = source["tradeDate"];
	        this.session = source["session"];
	        this.strategyName = source["strategyName"];
	        this.minScore = source["minScore"];
	        this.hitTotal = source["hitTotal"];
	        this.itemCount = source["itemCount"];
	        this.message = source["message"];
	        this.items = this.convertValues(source["items"], ResearchSnapshotCandidate);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SentimentResult {
	    Score: number;
	    Category: number;
	    PositiveCount: number;
	    NegativeCount: number;
	    Description: string;
	
	    static createFrom(source: any = {}) {
	        return new SentimentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Score = source["Score"];
	        this.Category = source["Category"];
	        this.PositiveCount = source["PositiveCount"];
	        this.NegativeCount = source["NegativeCount"];
	        this.Description = source["Description"];
	    }
	}
	export class SignalScanHit {
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE?: string;
	    CHANGE_RATE?: string;
	    HIGH_PRICE?: string;
	    LOW_PRICE?: string;
	    PRE_CLOSE_PRICE?: string;
	    VOLUME?: string;
	    DEAL_AMOUNT?: string;
	    TURNOVERRATE?: string;
	    VOLUME_RATIO?: string;
	    INDUSTRY?: string;
	    CONCEPT?: string;
	    MARKET?: string;
	    tag: string;
	    recentSignalDaysAgo?: number;
	    statusText: string;
	    sortRank: number;
	    rsi?: number;
	    schema_version?: string;
	    signal_price?: number;
	    signal_time?: string;
	    signal_price_source?: string;
	    signal_days_ago?: number;
	    signal_bar_role?: string;
	    signal_bar_index?: number;
	    confirm_bar_index?: number;
	    signal_price_status?: string;
	
	    static createFrom(source: any = {}) {
	        return new SignalScanHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.CONCEPT = source["CONCEPT"];
	        this.MARKET = source["MARKET"];
	        this.tag = source["tag"];
	        this.recentSignalDaysAgo = source["recentSignalDaysAgo"];
	        this.statusText = source["statusText"];
	        this.sortRank = source["sortRank"];
	        this.rsi = source["rsi"];
	        this.schema_version = source["schema_version"];
	        this.signal_price = source["signal_price"];
	        this.signal_time = source["signal_time"];
	        this.signal_price_source = source["signal_price_source"];
	        this.signal_days_ago = source["signal_days_ago"];
	        this.signal_bar_role = source["signal_bar_role"];
	        this.signal_bar_index = source["signal_bar_index"];
	        this.confirm_bar_index = source["confirm_bar_index"];
	        this.signal_price_status = source["signal_price_status"];
	    }
	}
	export class UniverseSignalSnapshotConfig {
	    strategyId?: number;
	    strategyRunId?: number;
	    universeId: string;
	    scope: string;
	    strategyKey?: string;
	
	    static createFrom(source: any = {}) {
	        return new UniverseSignalSnapshotConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategyId = source["strategyId"];
	        this.strategyRunId = source["strategyRunId"];
	        this.universeId = source["universeId"];
	        this.scope = source["scope"];
	        this.strategyKey = source["strategyKey"];
	    }
	}
	export class SignalScanResultPayload {
	    items: SignalScanHit[];
	    scannedTotal: number;
	    hitTotal: number;
	    tradeDate: string;
	    session: string;
	    strategyId?: string;
	    strategyName?: string;
	    completedAt: string;
	    config?: UniverseSignalSnapshotConfig;
	
	    static createFrom(source: any = {}) {
	        return new SignalScanResultPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], SignalScanHit);
	        this.scannedTotal = source["scannedTotal"];
	        this.hitTotal = source["hitTotal"];
	        this.tradeDate = source["tradeDate"];
	        this.session = source["session"];
	        this.strategyId = source["strategyId"];
	        this.strategyName = source["strategyName"];
	        this.completedAt = source["completedAt"];
	        this.config = this.convertValues(source["config"], UniverseSignalSnapshotConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SignalScanSnapshot {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    tradeDate: string;
	    session: string;
	    scope: string;
	    strategyId: string;
	    strategyName: string;
	    signalParamsJson: string;
	    scannedTotal: number;
	    hitTotal: number;
	    status: string;
	    message: string;
	    resultJson: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new SignalScanSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.tradeDate = source["tradeDate"];
	        this.session = source["session"];
	        this.scope = source["scope"];
	        this.strategyId = source["strategyId"];
	        this.strategyName = source["strategyName"];
	        this.signalParamsJson = source["signalParamsJson"];
	        this.scannedTotal = source["scannedTotal"];
	        this.hitTotal = source["hitTotal"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.resultJson = source["resultJson"];
	        this.durationMs = source["durationMs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SignalScanSnapshotPageResp {
	    total: number;
	    data: SignalScanSnapshot[];
	
	    static createFrom(source: any = {}) {
	        return new SignalScanSnapshotPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], SignalScanSnapshot);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SignalScanSnapshotQuery {
	    page: number;
	    pageSize: number;
	    tradeDate: string;
	    session: string;
	    strategyId: string;
	
	    static createFrom(source: any = {}) {
	        return new SignalScanSnapshotQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.tradeDate = source["tradeDate"];
	        this.session = source["session"];
	        this.strategyId = source["strategyId"];
	    }
	}
	export class StockChangeHistory {
	    id: number;
	    changeTime: string;
	    changeDate: string;
	    stockCode: string;
	    stockName: string;
	    market: number;
	    changeType: number;
	    typeName: string;
	    volume: number;
	    price: number;
	    changeRate: number;
	    amount: number;
	    industry: string;
	    concept: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.changeTime = source["changeTime"];
	        this.changeDate = source["changeDate"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.market = source["market"];
	        this.changeType = source["changeType"];
	        this.typeName = source["typeName"];
	        this.volume = source["volume"];
	        this.price = source["price"];
	        this.changeRate = source["changeRate"];
	        this.amount = source["amount"];
	        this.industry = source["industry"];
	        this.concept = source["concept"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockChangeHistoryPageData {
	    list: StockChangeHistory[];
	    total: number;
	    page: number;
	    pageSize: number;
	    totalPages: number;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistoryPageData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.list = this.convertValues(source["list"], StockChangeHistory);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.totalPages = source["totalPages"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockChangeHistoryQuery {
	    stockCode: string;
	    stockName: string;
	    changeType: number;
	    changeTypes: number[];
	    typeName: string;
	    startDate: string;
	    endDate: string;
	    startTime: string;
	    endTime: string;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new StockChangeHistoryQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.changeType = source["changeType"];
	        this.changeTypes = source["changeTypes"];
	        this.typeName = source["typeName"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class StockInfo {
	    SECUCODE: string;
	    SECURITY_CODE: string;
	    SECURITY_NAME_ABBR: string;
	    NEW_PRICE: any;
	    CHANGE_RATE: any;
	    VOLUME_RATIO: any;
	    HIGH_PRICE: any;
	    LOW_PRICE: any;
	    PRE_CLOSE_PRICE: any;
	    VOLUME: any;
	    DEAL_AMOUNT: any;
	    TURNOVERRATE: any;
	    MARKET: string;
	    CONCEPT: any;
	    INDUSTRY: string;
	    MAX_TRADE_DATE: string;
	
	    static createFrom(source: any = {}) {
	        return new StockInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.SECUCODE = source["SECUCODE"];
	        this.SECURITY_CODE = source["SECURITY_CODE"];
	        this.SECURITY_NAME_ABBR = source["SECURITY_NAME_ABBR"];
	        this.NEW_PRICE = source["NEW_PRICE"];
	        this.CHANGE_RATE = source["CHANGE_RATE"];
	        this.VOLUME_RATIO = source["VOLUME_RATIO"];
	        this.HIGH_PRICE = source["HIGH_PRICE"];
	        this.LOW_PRICE = source["LOW_PRICE"];
	        this.PRE_CLOSE_PRICE = source["PRE_CLOSE_PRICE"];
	        this.VOLUME = source["VOLUME"];
	        this.DEAL_AMOUNT = source["DEAL_AMOUNT"];
	        this.TURNOVERRATE = source["TURNOVERRATE"];
	        this.MARKET = source["MARKET"];
	        this.CONCEPT = source["CONCEPT"];
	        this.INDUSTRY = source["INDUSTRY"];
	        this.MAX_TRADE_DATE = source["MAX_TRADE_DATE"];
	    }
	}
	export class StockStrategy {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    name: string;
	    queryType: string;
	    queryText: string;
	    queryJson: string;
	    keyword: string;
	    industry: string;
	    cronExpr: string;
	    enable: boolean;
	    pageSize: number;
	    description: string;
	    // Go type: time
	    lastRunAt?: any;
	    lastRunCount: number;
	    lastRunError: string;
	
	    static createFrom(source: any = {}) {
	        return new StockStrategy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.name = source["name"];
	        this.queryType = source["queryType"];
	        this.queryText = source["queryText"];
	        this.queryJson = source["queryJson"];
	        this.keyword = source["keyword"];
	        this.industry = source["industry"];
	        this.cronExpr = source["cronExpr"];
	        this.enable = source["enable"];
	        this.pageSize = source["pageSize"];
	        this.description = source["description"];
	        this.lastRunAt = this.convertValues(source["lastRunAt"], null);
	        this.lastRunCount = source["lastRunCount"];
	        this.lastRunError = source["lastRunError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockStrategyPageResp {
	    total: number;
	    data: StockStrategy[];
	
	    static createFrom(source: any = {}) {
	        return new StockStrategyPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], StockStrategy);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockStrategyQuery {
	    page: number;
	    pageSize: number;
	    name: string;
	    queryType: string;
	
	    static createFrom(source: any = {}) {
	        return new StockStrategyQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	        this.name = source["name"];
	        this.queryType = source["queryType"];
	    }
	}
	export class StockStrategyRun {
	    id: number;
	    // Go type: time
	    createdAt: any;
	    strategyId: number;
	    stockCount: number;
	    message: string;
	    resultJson: string;
	
	    static createFrom(source: any = {}) {
	        return new StockStrategyRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.strategyId = source["strategyId"];
	        this.stockCount = source["stockCount"];
	        this.message = source["message"];
	        this.resultJson = source["resultJson"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockStrategyRunPageResp {
	    total: number;
	    data: StockStrategyRun[];
	
	    static createFrom(source: any = {}) {
	        return new StockStrategyRunPageResp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.data = this.convertValues(source["data"], StockStrategyRun);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StockStrategyRunQuery {
	    strategyId: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new StockStrategyRunQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategyId = source["strategyId"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
	    }
	}
	export class StockStrategyRunView {
	    runId: number;
	    strategyId: number;
	    code: number;
	    message: string;
	    queryType: string;
	    stockCount: number;
	    traceInfo?: string;
	    columns?: any;
	    dataList?: any;
	    runAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new StockStrategyRunView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.strategyId = source["strategyId"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.queryType = source["queryType"];
	        this.stockCount = source["stockCount"];
	        this.traceInfo = source["traceInfo"];
	        this.columns = source["columns"];
	        this.dataList = source["dataList"];
	        this.runAt = source["runAt"];
	    }
	}
	export class TechnicalIndicators {
	    MACD_GOLDEN_FORK: boolean;
	    KDJ_GOLDEN_FORK: boolean;
	    BREAK_THROUGH: boolean;
	    LOW_FUNDS_INFLOW: boolean;
	    HIGH_FUNDS_OUTFLOW: boolean;
	    BREAKUP_MA_5DAYS: boolean;
	    LONG_AVG_ARRAY: boolean;
	    SHORT_AVG_ARRAY: boolean;
	    UPPER_LARGE_VOLUME: boolean;
	    DOWN_NARROW_VOLUME: boolean;
	    ONE_DAYANG_LINE: boolean;
	    TWO_DAYANG_LINES: boolean;
	    RISE_SUN: boolean;
	    POWER_FULGUN: boolean;
	    RESTORE_JUSTICE: boolean;
	    DOWN_7DAYS: boolean;
	    UPPER_8DAYS: boolean;
	    UPPER_9DAYS: boolean;
	    UPPER_4DAYS: boolean;
	    HEAVEN_RULE: boolean;
	    UPSIDE_VOLUME: boolean;
	    BEARISH_ENGULFING: boolean;
	    REVERSING_HAMMER: boolean;
	    SHOOTING_STAR: boolean;
	    EVENING_STAR: boolean;
	    FIRST_DAWN: boolean;
	    PREGNANT: boolean;
	    BLACK_CLOUD_TOPS: boolean;
	    MORNING_STAR: boolean;
	    NARROW_FINISH: boolean;
	    UPP_DAYS: number;
	    CONCERN_RANK_7DAYS: number;
	    UPNDAY: number;
	    DOWNNDAY: number;
	
	    static createFrom(source: any = {}) {
	        return new TechnicalIndicators(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MACD_GOLDEN_FORK = source["MACD_GOLDEN_FORK"];
	        this.KDJ_GOLDEN_FORK = source["KDJ_GOLDEN_FORK"];
	        this.BREAK_THROUGH = source["BREAK_THROUGH"];
	        this.LOW_FUNDS_INFLOW = source["LOW_FUNDS_INFLOW"];
	        this.HIGH_FUNDS_OUTFLOW = source["HIGH_FUNDS_OUTFLOW"];
	        this.BREAKUP_MA_5DAYS = source["BREAKUP_MA_5DAYS"];
	        this.LONG_AVG_ARRAY = source["LONG_AVG_ARRAY"];
	        this.SHORT_AVG_ARRAY = source["SHORT_AVG_ARRAY"];
	        this.UPPER_LARGE_VOLUME = source["UPPER_LARGE_VOLUME"];
	        this.DOWN_NARROW_VOLUME = source["DOWN_NARROW_VOLUME"];
	        this.ONE_DAYANG_LINE = source["ONE_DAYANG_LINE"];
	        this.TWO_DAYANG_LINES = source["TWO_DAYANG_LINES"];
	        this.RISE_SUN = source["RISE_SUN"];
	        this.POWER_FULGUN = source["POWER_FULGUN"];
	        this.RESTORE_JUSTICE = source["RESTORE_JUSTICE"];
	        this.DOWN_7DAYS = source["DOWN_7DAYS"];
	        this.UPPER_8DAYS = source["UPPER_8DAYS"];
	        this.UPPER_9DAYS = source["UPPER_9DAYS"];
	        this.UPPER_4DAYS = source["UPPER_4DAYS"];
	        this.HEAVEN_RULE = source["HEAVEN_RULE"];
	        this.UPSIDE_VOLUME = source["UPSIDE_VOLUME"];
	        this.BEARISH_ENGULFING = source["BEARISH_ENGULFING"];
	        this.REVERSING_HAMMER = source["REVERSING_HAMMER"];
	        this.SHOOTING_STAR = source["SHOOTING_STAR"];
	        this.EVENING_STAR = source["EVENING_STAR"];
	        this.FIRST_DAWN = source["FIRST_DAWN"];
	        this.PREGNANT = source["PREGNANT"];
	        this.BLACK_CLOUD_TOPS = source["BLACK_CLOUD_TOPS"];
	        this.MORNING_STAR = source["MORNING_STAR"];
	        this.NARROW_FINISH = source["NARROW_FINISH"];
	        this.UPP_DAYS = source["UPP_DAYS"];
	        this.CONCERN_RANK_7DAYS = source["CONCERN_RANK_7DAYS"];
	        this.UPNDAY = source["UPNDAY"];
	        this.DOWNNDAY = source["DOWNNDAY"];
	    }
	}
	export class TradePlanItem {
	    id: number;
	    planId: number;
	    tradeDate: string;
	    stockCode: string;
	    stockName: string;
	    side: string;
	    priority: number;
	    targetAmount: number;
	    targetVolume: number;
	    limitPrice: number;
	    score: number;
	    reason: string;
	    strategyName: string;
	    strategyVersion: string;
	    status: string;
	    riskCode: string;
	    riskMessage: string;
	    error: string;
	    orderId: number;
	    fillId: number;
	    filledPrice: number;
	    filledVolume: number;
	    filledFee: number;
	    originalTargetAmount: number;
	    refPrice: number;
	    refSource: string;
	    refAsOf: string;
	    entryRule: string;
	    maxSlippage?: number;
	    intentStatus: string;
	    openRefPrice: number;
	    // Go type: time
	    pricedAt?: any;
	    pricedBy: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TradePlanItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.planId = source["planId"];
	        this.tradeDate = source["tradeDate"];
	        this.stockCode = source["stockCode"];
	        this.stockName = source["stockName"];
	        this.side = source["side"];
	        this.priority = source["priority"];
	        this.targetAmount = source["targetAmount"];
	        this.targetVolume = source["targetVolume"];
	        this.limitPrice = source["limitPrice"];
	        this.score = source["score"];
	        this.reason = source["reason"];
	        this.strategyName = source["strategyName"];
	        this.strategyVersion = source["strategyVersion"];
	        this.status = source["status"];
	        this.riskCode = source["riskCode"];
	        this.riskMessage = source["riskMessage"];
	        this.error = source["error"];
	        this.orderId = source["orderId"];
	        this.fillId = source["fillId"];
	        this.filledPrice = source["filledPrice"];
	        this.filledVolume = source["filledVolume"];
	        this.filledFee = source["filledFee"];
	        this.originalTargetAmount = source["originalTargetAmount"];
	        this.refPrice = source["refPrice"];
	        this.refSource = source["refSource"];
	        this.refAsOf = source["refAsOf"];
	        this.entryRule = source["entryRule"];
	        this.maxSlippage = source["maxSlippage"];
	        this.intentStatus = source["intentStatus"];
	        this.openRefPrice = source["openRefPrice"];
	        this.pricedAt = this.convertValues(source["pricedAt"], null);
	        this.pricedBy = source["pricedBy"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TradePlan {
	    id: number;
	    tradeDate: string;
	    poolId: number;
	    // Go type: time
	    generatedAt: any;
	    status: string;
	    side: string;
	    amountPerStock: number;
	    maxNames: number;
	    enableExecute: boolean;
	    message: string;
	    planVersion: number;
	    // Go type: time
	    freezeAt?: any;
	    freezeBy: string;
	    freezeReason: string;
	    // Go type: time
	    approvedAt?: any;
	    approvedBy: string;
	    approvalReason: string;
	    approvedSource: string;
	    sourceSession: string;
	    parentPlanId: number;
	    sourceKind: string;
	    rescaleMode: string;
	    availableCashUsed: number;
	    requiredCashBefore: number;
	    requiredCashAfter: number;
	    scaleRatio: number;
	    providerMode: string;
	    decisionProvider: string;
	    decisionVersion: string;
	    allocationVersion: string;
	    defaultEntryRule: string;
	    defaultMaxSlippage?: number;
	    pricingPolicyVersion: number;
	    pricingStage: string;
	    riskStatus: string;
	    marketLevel: number;
	    riskFilteredCount: number;
	    riskAcceptedCount: number;
	    riskSummary: string;
	    riskSnapshotJson: string;
	    // Go type: time
	    checkedAt?: any;
	    // Go type: time
	    executedAt?: any;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    items?: TradePlanItem[];
	
	    static createFrom(source: any = {}) {
	        return new TradePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.tradeDate = source["tradeDate"];
	        this.poolId = source["poolId"];
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
	        this.status = source["status"];
	        this.side = source["side"];
	        this.amountPerStock = source["amountPerStock"];
	        this.maxNames = source["maxNames"];
	        this.enableExecute = source["enableExecute"];
	        this.message = source["message"];
	        this.planVersion = source["planVersion"];
	        this.freezeAt = this.convertValues(source["freezeAt"], null);
	        this.freezeBy = source["freezeBy"];
	        this.freezeReason = source["freezeReason"];
	        this.approvedAt = this.convertValues(source["approvedAt"], null);
	        this.approvedBy = source["approvedBy"];
	        this.approvalReason = source["approvalReason"];
	        this.approvedSource = source["approvedSource"];
	        this.sourceSession = source["sourceSession"];
	        this.parentPlanId = source["parentPlanId"];
	        this.sourceKind = source["sourceKind"];
	        this.rescaleMode = source["rescaleMode"];
	        this.availableCashUsed = source["availableCashUsed"];
	        this.requiredCashBefore = source["requiredCashBefore"];
	        this.requiredCashAfter = source["requiredCashAfter"];
	        this.scaleRatio = source["scaleRatio"];
	        this.providerMode = source["providerMode"];
	        this.decisionProvider = source["decisionProvider"];
	        this.decisionVersion = source["decisionVersion"];
	        this.allocationVersion = source["allocationVersion"];
	        this.defaultEntryRule = source["defaultEntryRule"];
	        this.defaultMaxSlippage = source["defaultMaxSlippage"];
	        this.pricingPolicyVersion = source["pricingPolicyVersion"];
	        this.pricingStage = source["pricingStage"];
	        this.riskStatus = source["riskStatus"];
	        this.marketLevel = source["marketLevel"];
	        this.riskFilteredCount = source["riskFilteredCount"];
	        this.riskAcceptedCount = source["riskAcceptedCount"];
	        this.riskSummary = source["riskSummary"];
	        this.riskSnapshotJson = source["riskSnapshotJson"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.executedAt = this.convertValues(source["executedAt"], null);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.items = this.convertValues(source["items"], TradePlanItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class VersionInfo {
	    ID: number;
	    // Go type: time
	    CreatedAt: any;
	    // Go type: time
	    UpdatedAt: any;
	    // Go type: gorm
	    DeletedAt: any;
	    version: string;
	    content: string;
	    icon: string;
	    alipay: string;
	    wxpay: string;
	    wxgzh: string;
	    buildTimeStamp: number;
	    officialStatement: string;
	    IsDel: number;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.CreatedAt = this.convertValues(source["CreatedAt"], null);
	        this.UpdatedAt = this.convertValues(source["UpdatedAt"], null);
	        this.DeletedAt = this.convertValues(source["DeletedAt"], null);
	        this.version = source["version"];
	        this.content = source["content"];
	        this.icon = source["icon"];
	        this.alipay = source["alipay"];
	        this.wxpay = source["wxpay"];
	        this.wxgzh = source["wxgzh"];
	        this.buildTimeStamp = source["buildTimeStamp"];
	        this.officialStatement = source["officialStatement"];
	        this.IsDel = source["IsDel"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace papertrading {
	
	export class DevAccountResult {
	    ok: boolean;
	    operation: string;
	    accountId: number;
	    name: string;
	    cash: number;
	    initialCash: number;
	    equity: number;
	    marketValue: number;
	    message: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new DevAccountResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.operation = source["operation"];
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.cash = source["cash"];
	        this.initialCash = source["initialCash"];
	        this.equity = source["equity"];
	        this.marketValue = source["marketValue"];
	        this.message = source["message"];
	        this.error = source["error"];
	    }
	}

}

export namespace research {
	
	export class Candidate {
	    id: string;
	    trade_date: string;
	    stock_code: string;
	    stock_name: string;
	    status: string;
	    source: string;
	    source_ref?: string;
	    signal_tag?: string;
	    signal_score?: number;
	    signal_snapshot_id?: number;
	    score?: number;
	    rank?: number;
	    tags?: string[];
	    note?: string;
	    // Go type: time
	    updated_at?: any;
	    promoted_pool_id?: number;
	    // Go type: time
	    promoted_at?: any;
	    explain_ref?: string;
	    explain_summary?: string;
	    direction?: string;
	    price?: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.trade_date = source["trade_date"];
	        this.stock_code = source["stock_code"];
	        this.stock_name = source["stock_name"];
	        this.status = source["status"];
	        this.source = source["source"];
	        this.source_ref = source["source_ref"];
	        this.signal_tag = source["signal_tag"];
	        this.signal_score = source["signal_score"];
	        this.signal_snapshot_id = source["signal_snapshot_id"];
	        this.score = source["score"];
	        this.rank = source["rank"];
	        this.tags = source["tags"];
	        this.note = source["note"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.promoted_pool_id = source["promoted_pool_id"];
	        this.promoted_at = this.convertValues(source["promoted_at"], null);
	        this.explain_ref = source["explain_ref"];
	        this.explain_summary = source["explain_summary"];
	        this.direction = source["direction"];
	        this.price = source["price"];
	        this.reason = source["reason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LinksShell {
	    trade_pool_id?: number;
	    trade_plan_id?: number;
	
	    static createFrom(source: any = {}) {
	        return new LinksShell(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trade_pool_id = source["trade_pool_id"];
	        this.trade_plan_id = source["trade_plan_id"];
	    }
	}
	export class ExplanationShell {
	    available: boolean;
	    summary: string;
	    missing_reason?: string;
	    explain_ref?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExplanationShell(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.summary = source["summary"];
	        this.missing_reason = source["missing_reason"];
	        this.explain_ref = source["explain_ref"];
	    }
	}
	export class DetailResult {
	    candidate: Candidate;
	    explanation: ExplanationShell;
	    links: LinksShell;
	
	    static createFrom(source: any = {}) {
	        return new DetailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidate = this.convertValues(source["candidate"], Candidate);
	        this.explanation = this.convertValues(source["explanation"], ExplanationShell);
	        this.links = this.convertValues(source["links"], LinksShell);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RiskNote {
	    severity: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.text = source["text"];
	    }
	}
	export class ResearchReason {
	    kind: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new ResearchReason(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.text = source["text"];
	    }
	}
	export class ExplainEvidence {
	    as_of?: string;
	    signal_snapshot_id?: number;
	    source?: string;
	    source_ref?: string;
	    signal_tag?: string;
	    signal_score?: number;
	    direction?: string;
	    price?: string;
	    status_text?: string;
	    score_steps?: string[];
	    evidence_hash?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExplainEvidence(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.as_of = source["as_of"];
	        this.signal_snapshot_id = source["signal_snapshot_id"];
	        this.source = source["source"];
	        this.source_ref = source["source_ref"];
	        this.signal_tag = source["signal_tag"];
	        this.signal_score = source["signal_score"];
	        this.direction = source["direction"];
	        this.price = source["price"];
	        this.status_text = source["status_text"];
	        this.score_steps = source["score_steps"];
	        this.evidence_hash = source["evidence_hash"];
	    }
	}
	export class Explain {
	    id: string;
	    schema_version: string;
	    candidate_id: string;
	    trade_date?: string;
	    stock_code?: string;
	    explain_type: string;
	    available: boolean;
	    missing_reason?: string;
	    summary: string;
	    evidence: ExplainEvidence;
	    research_reason: ResearchReason;
	    risk_note?: RiskNote;
	    strategy_intent_ref?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Explain(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.schema_version = source["schema_version"];
	        this.candidate_id = source["candidate_id"];
	        this.trade_date = source["trade_date"];
	        this.stock_code = source["stock_code"];
	        this.explain_type = source["explain_type"];
	        this.available = source["available"];
	        this.missing_reason = source["missing_reason"];
	        this.summary = source["summary"];
	        this.evidence = this.convertValues(source["evidence"], ExplainEvidence);
	        this.research_reason = this.convertValues(source["research_reason"], ResearchReason);
	        this.risk_note = this.convertValues(source["risk_note"], RiskNote);
	        this.strategy_intent_ref = source["strategy_intent_ref"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class ListResult {
	    trade_date: string;
	    items: Candidate[];
	    as_of: string;
	    message?: string;
	    threshold?: number;
	    snapshot_id?: number;
	
	    static createFrom(source: any = {}) {
	        return new ListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trade_date = source["trade_date"];
	        this.items = this.convertValues(source["items"], Candidate);
	        this.as_of = source["as_of"];
	        this.message = source["message"];
	        this.threshold = source["threshold"];
	        this.snapshot_id = source["snapshot_id"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace risk {
	
	export class Metrics {
	    totalAssets: number;
	    totalLiabilities: number;
	    netExposure: number;
	    grossExposure: number;
	    maintenanceRatio: number;
	    marginAvailable: number;
	    requiredOrderBond: number;
	    grossExposurePct: number;
	    singleNamePct: number;
	
	    static createFrom(source: any = {}) {
	        return new Metrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalAssets = source["totalAssets"];
	        this.totalLiabilities = source["totalLiabilities"];
	        this.netExposure = source["netExposure"];
	        this.grossExposure = source["grossExposure"];
	        this.maintenanceRatio = source["maintenanceRatio"];
	        this.marginAvailable = source["marginAvailable"];
	        this.requiredOrderBond = source["requiredOrderBond"];
	        this.grossExposurePct = source["grossExposurePct"];
	        this.singleNamePct = source["singleNamePct"];
	    }
	}
	export class RiskDecision {
	    allowed: boolean;
	    code: string;
	    message: string;
	    metrics: Metrics;
	
	    static createFrom(source: any = {}) {
	        return new RiskDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed = source["allowed"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.metrics = this.convertValues(source["metrics"], Metrics);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace riskreport {
	
	export class DimensionSlice {
	    score: number;
	    factor_count: number;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DimensionSlice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.score = source["score"];
	        this.factor_count = source["factor_count"];
	        this.available = source["available"];
	    }
	}
	export class DimensionsView {
	    portfolio: DimensionSlice;
	    position: DimensionSlice;
	    execution: DimensionSlice;
	    market: DimensionSlice;
	
	    static createFrom(source: any = {}) {
	        return new DimensionsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.portfolio = this.convertValues(source["portfolio"], DimensionSlice);
	        this.position = this.convertValues(source["position"], DimensionSlice);
	        this.execution = this.convertValues(source["execution"], DimensionSlice);
	        this.market = this.convertValues(source["market"], DimensionSlice);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RiskFactor {
	    code: string;
	    dimension: string;
	    severity: string;
	    title: string;
	    detail: string;
	    metric_value?: number;
	    metric_unit?: string;
	    related_symbols?: string[];
	
	    static createFrom(source: any = {}) {
	        return new RiskFactor(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.dimension = source["dimension"];
	        this.severity = source["severity"];
	        this.title = source["title"];
	        this.detail = source["detail"];
	        this.metric_value = source["metric_value"];
	        this.metric_unit = source["metric_unit"];
	        this.related_symbols = source["related_symbols"];
	    }
	}
	export class SourceRef {
	    kind: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new SourceRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.label = source["label"];
	    }
	}
	export class RiskSuggestion {
	    code: string;
	    message: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.kind = source["kind"];
	    }
	}
	export class RiskWarning {
	    code: string;
	    message: string;
	    factor_codes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new RiskWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	        this.factor_codes = source["factor_codes"];
	    }
	}
	export class RiskScore {
	    overall: number;
	    band: string;
	    by_dimension: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new RiskScore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall = source["overall"];
	        this.band = source["band"];
	        this.by_dimension = source["by_dimension"];
	    }
	}
	export class RiskReport {
	    report_id: string;
	    schema_version: string;
	    user_id?: string;
	    trade_date?: string;
	    account_scope?: string;
	    // Go type: time
	    generated_at: any;
	    status: string;
	    gate_reason?: string;
	    score: RiskScore;
	    factors: RiskFactor[];
	    warnings: RiskWarning[];
	    suggestions: RiskSuggestion[];
	    dimensions: DimensionsView;
	    sources?: SourceRef[];
	    data_quality: string;
	    disclaimers: string[];
	
	    static createFrom(source: any = {}) {
	        return new RiskReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report_id = source["report_id"];
	        this.schema_version = source["schema_version"];
	        this.user_id = source["user_id"];
	        this.trade_date = source["trade_date"];
	        this.account_scope = source["account_scope"];
	        this.generated_at = this.convertValues(source["generated_at"], null);
	        this.status = source["status"];
	        this.gate_reason = source["gate_reason"];
	        this.score = this.convertValues(source["score"], RiskScore);
	        this.factors = this.convertValues(source["factors"], RiskFactor);
	        this.warnings = this.convertValues(source["warnings"], RiskWarning);
	        this.suggestions = this.convertValues(source["suggestions"], RiskSuggestion);
	        this.dimensions = this.convertValues(source["dimensions"], DimensionsView);
	        this.sources = this.convertValues(source["sources"], SourceRef);
	        this.data_quality = source["data_quality"];
	        this.disclaimers = source["disclaimers"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace strategyexplain {
	
	export class Citation {
	    kind: string;
	    ref: string;
	    label?: string;
	
	    static createFrom(source: any = {}) {
	        return new Citation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.ref = source["ref"];
	        this.label = source["label"];
	    }
	}
	export class EntryReason {
	    strategy_name?: string;
	    strategy_version?: string;
	    entry_reason_raw?: string;
	    entry_rule?: string;
	    ref_price?: number;
	    ref_source?: string;
	    narrative?: string;
	    thesis_intact_note?: string;
	
	    static createFrom(source: any = {}) {
	        return new EntryReason(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy_name = source["strategy_name"];
	        this.strategy_version = source["strategy_version"];
	        this.entry_reason_raw = source["entry_reason_raw"];
	        this.entry_rule = source["entry_rule"];
	        this.ref_price = source["ref_price"];
	        this.ref_source = source["ref_source"];
	        this.narrative = source["narrative"];
	        this.thesis_intact_note = source["thesis_intact_note"];
	    }
	}
	export class ExitReason {
	    mode: string;
	    exit_state?: string;
	    reason_codes?: string[];
	    narrative?: string;
	    compared_to_entry?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExitReason(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.exit_state = source["exit_state"];
	        this.reason_codes = source["reason_codes"];
	        this.narrative = source["narrative"];
	        this.compared_to_entry = source["compared_to_entry"];
	    }
	}
	export class RiskExplanation {
	    available: boolean;
	    accepted?: boolean;
	    risk_code?: string;
	    risk_message?: string;
	    plan_status?: string;
	    narrative?: string;
	    missing_reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.accepted = source["accepted"];
	        this.risk_code = source["risk_code"];
	        this.risk_message = source["risk_message"];
	        this.plan_status = source["plan_status"];
	        this.narrative = source["narrative"];
	        this.missing_reason = source["missing_reason"];
	    }
	}
	export class SignalExplanation {
	    available: boolean;
	    tag?: string;
	    score?: number;
	    snapshot_ref?: number;
	    narrative?: string;
	    missing_reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new SignalExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.tag = source["tag"];
	        this.score = source["score"];
	        this.snapshot_ref = source["snapshot_ref"];
	        this.narrative = source["narrative"];
	        this.missing_reason = source["missing_reason"];
	    }
	}
	export class ExplanationSections {
	    signal: SignalExplanation;
	    risk: RiskExplanation;
	    entry: EntryReason;
	    exit?: ExitReason;
	
	    static createFrom(source: any = {}) {
	        return new ExplanationSections(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signal = this.convertValues(source["signal"], SignalExplanation);
	        this.risk = this.convertValues(source["risk"], RiskExplanation);
	        this.entry = this.convertValues(source["entry"], EntryReason);
	        this.exit = this.convertValues(source["exit"], ExitReason);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Explanation {
	    explanation_id: string;
	    schema_version: string;
	    snapshot_id?: string;
	    plan_id?: number;
	    plan_item_id?: number;
	    trade_date?: string;
	    // Go type: time
	    generated_at: any;
	    status: string;
	    gate_reason?: string;
	    headline: string;
	    sections: ExplanationSections;
	    citations?: Citation[];
	    disclaimers: string[];
	
	    static createFrom(source: any = {}) {
	        return new Explanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.explanation_id = source["explanation_id"];
	        this.schema_version = source["schema_version"];
	        this.snapshot_id = source["snapshot_id"];
	        this.plan_id = source["plan_id"];
	        this.plan_item_id = source["plan_item_id"];
	        this.trade_date = source["trade_date"];
	        this.generated_at = this.convertValues(source["generated_at"], null);
	        this.status = source["status"];
	        this.gate_reason = source["gate_reason"];
	        this.headline = source["headline"];
	        this.sections = this.convertValues(source["sections"], ExplanationSections);
	        this.citations = this.convertValues(source["citations"], Citation);
	        this.disclaimers = source["disclaimers"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace strategyintent {
	
	export class SizingIntentUpper {
	    kind: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new SizingIntentUpper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.value = source["value"];
	    }
	}
	export class Action {
	    verb: string;
	    side_hint?: string;
	    entry_session?: string;
	    sizing_intent_upper?: SizingIntentUpper;
	    text?: string;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.verb = source["verb"];
	        this.side_hint = source["side_hint"];
	        this.entry_session = source["entry_session"];
	        this.sizing_intent_upper = this.convertValues(source["sizing_intent_upper"], SizingIntentUpper);
	        this.text = source["text"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Conditions {
	    trade_date?: string;
	    session?: string;
	    min_signal_score?: number;
	    valid_until?: string;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new Conditions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trade_date = source["trade_date"];
	        this.session = source["session"];
	        this.min_signal_score = source["min_signal_score"];
	        this.valid_until = source["valid_until"];
	        this.notes = source["notes"];
	    }
	}
	export class RiskConstraints {
	    severity_cap?: string;
	    avoid_if?: string[];
	    max_notional_hint?: number;
	    text?: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskConstraints(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity_cap = source["severity_cap"];
	        this.avoid_if = source["avoid_if"];
	        this.max_notional_hint = source["max_notional_hint"];
	        this.text = source["text"];
	    }
	}
	export class SchemaRef {
	    unbound: boolean;
	    strategy_id?: string;
	    revision?: string;
	    params_hash?: string;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new SchemaRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unbound = source["unbound"];
	        this.strategy_id = source["strategy_id"];
	        this.revision = source["revision"];
	        this.params_hash = source["params_hash"];
	        this.note = source["note"];
	    }
	}
	export class CreateDraftRequest {
	    candidate_id: string;
	    explain_ref?: string;
	    strategy_schema_ref: SchemaRef;
	    schema_revision?: string;
	    intent_type?: string;
	    summary?: string;
	    conditions: Conditions;
	    action: Action;
	    risk_constraints: RiskConstraints;
	
	    static createFrom(source: any = {}) {
	        return new CreateDraftRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidate_id = source["candidate_id"];
	        this.explain_ref = source["explain_ref"];
	        this.strategy_schema_ref = this.convertValues(source["strategy_schema_ref"], SchemaRef);
	        this.schema_revision = source["schema_revision"];
	        this.intent_type = source["intent_type"];
	        this.summary = source["summary"];
	        this.conditions = this.convertValues(source["conditions"], Conditions);
	        this.action = this.convertValues(source["action"], Action);
	        this.risk_constraints = this.convertValues(source["risk_constraints"], RiskConstraints);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Intent {
	    id: string;
	    schema_version: string;
	    candidate_id: string;
	    strategy_schema_ref: SchemaRef;
	    schema_revision?: string;
	    intent_type: string;
	    conditions: Conditions;
	    action: Action;
	    risk_constraints: RiskConstraints;
	    status: string;
	    summary?: string;
	    explain_ref?: string;
	    promoted_pool_id?: number;
	    trade_plan_id?: number;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Intent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.schema_version = source["schema_version"];
	        this.candidate_id = source["candidate_id"];
	        this.strategy_schema_ref = this.convertValues(source["strategy_schema_ref"], SchemaRef);
	        this.schema_revision = source["schema_revision"];
	        this.intent_type = source["intent_type"];
	        this.conditions = this.convertValues(source["conditions"], Conditions);
	        this.action = this.convertValues(source["action"], Action);
	        this.risk_constraints = this.convertValues(source["risk_constraints"], RiskConstraints);
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.explain_ref = source["explain_ref"];
	        this.promoted_pool_id = source["promoted_pool_id"];
	        this.trade_plan_id = source["trade_plan_id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ListItem {
	    id: string;
	    candidate_id: string;
	    status: string;
	    intent_type: string;
	    summary?: string;
	    schema_revision?: string;
	    strategy_id?: string;
	    unbound: boolean;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.candidate_id = source["candidate_id"];
	        this.status = source["status"];
	        this.intent_type = source["intent_type"];
	        this.summary = source["summary"];
	        this.schema_revision = source["schema_revision"];
	        this.strategy_id = source["strategy_id"];
	        this.unbound = source["unbound"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class ListResult {
	    items: ListItem[];
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], ListItem);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class UpdateDraftRequest {
	    explain_ref?: string;
	    strategy_schema_ref?: SchemaRef;
	    schema_revision?: string;
	    intent_type?: string;
	    summary?: string;
	    conditions?: Conditions;
	    action?: Action;
	    risk_constraints?: RiskConstraints;
	
	    static createFrom(source: any = {}) {
	        return new UpdateDraftRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.explain_ref = source["explain_ref"];
	        this.strategy_schema_ref = this.convertValues(source["strategy_schema_ref"], SchemaRef);
	        this.schema_revision = source["schema_revision"];
	        this.intent_type = source["intent_type"];
	        this.summary = source["summary"];
	        this.conditions = this.convertValues(source["conditions"], Conditions);
	        this.action = this.convertValues(source["action"], Action);
	        this.risk_constraints = this.convertValues(source["risk_constraints"], RiskConstraints);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WriteResult {
	    intent: Intent;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new WriteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.intent = this.convertValues(source["intent"], Intent);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace strategyschema {
	
	export class RiskProfileRef {
	    mode: string;
	    named_id?: string;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new RiskProfileRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.named_id = source["named_id"];
	        this.notes = source["notes"];
	    }
	}
	export class RankingSpec {
	    sort_by?: string[];
	    top_n?: number;
	    dedupe?: boolean;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new RankingSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sort_by = source["sort_by"];
	        this.top_n = source["top_n"];
	        this.dedupe = source["dedupe"];
	        this.notes = source["notes"];
	    }
	}
	export class FiltersSpec {
	    notes?: string;
	    rules?: any[];
	
	    static createFrom(source: any = {}) {
	        return new FiltersSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notes = source["notes"];
	        this.rules = source["rules"];
	    }
	}
	export class SignalsSpec {
	    kind?: string;
	    notes?: string;
	    compiled_ref?: string;
	    rules?: any[];
	
	    static createFrom(source: any = {}) {
	        return new SignalsSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.notes = source["notes"];
	        this.compiled_ref = source["compiled_ref"];
	        this.rules = source["rules"];
	    }
	}
	export class UniverseSpec {
	    source?: string;
	    adapter_ref?: string;
	    include?: string[];
	    exclude?: string[];
	    exclude_st?: boolean;
	    max_size?: number;
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new UniverseSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.adapter_ref = source["adapter_ref"];
	        this.include = source["include"];
	        this.exclude = source["exclude"];
	        this.exclude_st = source["exclude_st"];
	        this.max_size = source["max_size"];
	        this.notes = source["notes"];
	    }
	}
	export class CreateDraftRequest {
	    strategy_id?: string;
	    slug?: string;
	    name: string;
	    description?: string;
	    parent_revision?: string;
	    revision_note?: string;
	    name_override?: string;
	    universe: UniverseSpec;
	    signals: SignalsSpec;
	    filters: FiltersSpec;
	    ranking: RankingSpec;
	    risk_profile_ref: RiskProfileRef;
	    knobs?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new CreateDraftRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy_id = source["strategy_id"];
	        this.slug = source["slug"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.parent_revision = source["parent_revision"];
	        this.revision_note = source["revision_note"];
	        this.name_override = source["name_override"];
	        this.universe = this.convertValues(source["universe"], UniverseSpec);
	        this.signals = this.convertValues(source["signals"], SignalsSpec);
	        this.filters = this.convertValues(source["filters"], FiltersSpec);
	        this.ranking = this.convertValues(source["ranking"], RankingSpec);
	        this.risk_profile_ref = this.convertValues(source["risk_profile_ref"], RiskProfileRef);
	        this.knobs = source["knobs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Definition {
	    strategy_id: string;
	    schema_version: string;
	    name: string;
	    description?: string;
	    status: string;
	    current_revision?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Definition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy_id = source["strategy_id"];
	        this.schema_version = source["schema_version"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.current_revision = source["current_revision"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ParametersSpec {
	    knobs?: Record<string, any>;
	    params_hash?: string;
	
	    static createFrom(source: any = {}) {
	        return new ParametersSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.knobs = source["knobs"];
	        this.params_hash = source["params_hash"];
	    }
	}
	export class Revision {
	    revision_id: string;
	    strategy_id: string;
	    revision: string;
	    schema_version: string;
	    status: string;
	    name_override?: string;
	    revision_note?: string;
	    universe: UniverseSpec;
	    signals: SignalsSpec;
	    filters: FiltersSpec;
	    ranking: RankingSpec;
	    risk_profile_ref: RiskProfileRef;
	    parameters: ParametersSpec;
	    revision_hash?: string;
	    source?: string;
	    parent_revision?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    // Go type: time
	    activated_at?: any;
	    // Go type: time
	    retired_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new Revision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revision_id = source["revision_id"];
	        this.strategy_id = source["strategy_id"];
	        this.revision = source["revision"];
	        this.schema_version = source["schema_version"];
	        this.status = source["status"];
	        this.name_override = source["name_override"];
	        this.revision_note = source["revision_note"];
	        this.universe = this.convertValues(source["universe"], UniverseSpec);
	        this.signals = this.convertValues(source["signals"], SignalsSpec);
	        this.filters = this.convertValues(source["filters"], FiltersSpec);
	        this.ranking = this.convertValues(source["ranking"], RankingSpec);
	        this.risk_profile_ref = this.convertValues(source["risk_profile_ref"], RiskProfileRef);
	        this.parameters = this.convertValues(source["parameters"], ParametersSpec);
	        this.revision_hash = source["revision_hash"];
	        this.source = source["source"];
	        this.parent_revision = source["parent_revision"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.activated_at = this.convertValues(source["activated_at"], null);
	        this.retired_at = this.convertValues(source["retired_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RevisionSummary {
	    revision_id: string;
	    revision: string;
	    status: string;
	    revision_note?: string;
	    params_hash?: string;
	    revision_hash?: string;
	    source?: string;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new RevisionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revision_id = source["revision_id"];
	        this.revision = source["revision"];
	        this.status = source["status"];
	        this.revision_note = source["revision_note"];
	        this.params_hash = source["params_hash"];
	        this.revision_hash = source["revision_hash"];
	        this.source = source["source"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DetailResult {
	    definition: Definition;
	    revisions: RevisionSummary[];
	    current?: Revision;
	
	    static createFrom(source: any = {}) {
	        return new DetailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.definition = this.convertValues(source["definition"], Definition);
	        this.revisions = this.convertValues(source["revisions"], RevisionSummary);
	        this.current = this.convertValues(source["current"], Revision);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ListItem {
	    strategy_id: string;
	    name: string;
	    description?: string;
	    status: string;
	    current_revision?: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new ListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy_id = source["strategy_id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.current_revision = source["current_revision"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class ListResult {
	    items: ListItem[];
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], ListItem);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class RevisionListResult {
	    strategy_id: string;
	    items: RevisionSummary[];
	
	    static createFrom(source: any = {}) {
	        return new RevisionListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy_id = source["strategy_id"];
	        this.items = this.convertValues(source["items"], RevisionSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class UpdateDraftRequest {
	    revision_note?: string;
	    name_override?: string;
	    universe?: UniverseSpec;
	    signals?: SignalsSpec;
	    filters?: FiltersSpec;
	    ranking?: RankingSpec;
	    risk_profile_ref?: RiskProfileRef;
	    knobs?: Record<string, any>;
	    clear_knobs?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateDraftRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revision_note = source["revision_note"];
	        this.name_override = source["name_override"];
	        this.universe = this.convertValues(source["universe"], UniverseSpec);
	        this.signals = this.convertValues(source["signals"], SignalsSpec);
	        this.filters = this.convertValues(source["filters"], FiltersSpec);
	        this.ranking = this.convertValues(source["ranking"], RankingSpec);
	        this.risk_profile_ref = this.convertValues(source["risk_profile_ref"], RiskProfileRef);
	        this.knobs = source["knobs"];
	        this.clear_knobs = source["clear_knobs"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class WriteResult {
	    definition: Definition;
	    revision: Revision;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new WriteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.definition = this.convertValues(source["definition"], Definition);
	        this.revision = this.convertValues(source["revision"], Revision);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace version {
	
	export class CheckResult {
	    current_version: string;
	    latest_version: string;
	    update_available: boolean;
	    status: string;
	    release_note: string;
	    download_url: string;
	    channel?: string;
	    provider?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.update_available = source["update_available"];
	        this.status = source["status"];
	        this.release_note = source["release_note"];
	        this.download_url = source["download_url"];
	        this.channel = source["channel"];
	        this.provider = source["provider"];
	        this.error = source["error"];
	    }
	}
	export class VersionInfo {
	    version: string;
	    build_time: string;
	    git_commit: string;
	    commit_hash: string;
	    channel: string;
	    build_mode: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.build_time = source["build_time"];
	        this.git_commit = source["git_commit"];
	        this.commit_hash = source["commit_hash"];
	        this.channel = source["channel"];
	        this.build_mode = source["build_mode"];
	    }
	}

}

