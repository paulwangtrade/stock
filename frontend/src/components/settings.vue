<script setup>
import {computed, h, onBeforeUnmount, onMounted, ref} from "vue";
import {
  AddPrompt,
  DelPrompt,
  ExportConfig,
  GetConfig,
  GetPromptTemplates,
  SendDingDingMessageByType,
  UpdateConfig,
  FetchAiModels
} from "../../wailsjs/go/main/App";
import {NButton, NInput, NSelect, NSpace, NTag, NText, NTooltip, NIcon, useMessage} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";
import SignalSettingsPanel from "./SignalSettingsPanel.vue";
import QuantAutomationPanel from "./QuantAutomationPanel.vue";
import {
  applySignalParamsFromConfig,
  signalSettingsState,
} from "../utils/signalSettingsStore";
import {
  DEFAULT_SCREEN_STRATEGY_ID,
  extractSignalStrategySettings,
  mergeSignalSettings,
  serializeSignalParams,
  setActiveScreenStrategy,
} from "../utils/signalSettings";

const message = useMessage()

const formRef = ref(null)
const activeStrategyId = computed({
  get: () => mergeSignalSettings(signalSettingsState.value).activeScreenStrategyId,
  set: (id) => {
    signalSettingsState.value = setActiveScreenStrategy(signalSettingsState.value, id)
  },
})
const strategyOptions = computed(() =>
  mergeSignalSettings(signalSettingsState.value).screenStrategies.map((item) => ({
    label: item.name,
    value: item.id,
  })),
)
const activeStrategySettings = computed({
  get: () => {
    const s = mergeSignalSettings(signalSettingsState.value)
    return s.screenStrategies.find((item) => item.id === s.activeScreenStrategyId)?.settings || extractSignalStrategySettings(s)
  },
  set: (val) => {
    const s = mergeSignalSettings(signalSettingsState.value)
    const idx = s.screenStrategies.findIndex((item) => item.id === s.activeScreenStrategyId)
    if (idx >= 0) {
      s.screenStrategies[idx] = {
        ...s.screenStrategies[idx],
        settings: extractSignalStrategySettings(val),
      }
    }
    signalSettingsState.value = s
  },
})
const globalDisplaySettings = computed({
  get: () => mergeSignalSettings(signalSettingsState.value),
  set: (val) => {
    const current = mergeSignalSettings(signalSettingsState.value)
    const next = mergeSignalSettings(val)
    signalSettingsState.value = {
      ...current,
      display: next.display,
    }
  },
})
const activeStrategyName = computed({
  get: () => {
    const s = mergeSignalSettings(signalSettingsState.value)
    return s.screenStrategies.find((item) => item.id === s.activeScreenStrategyId)?.name || ''
  },
  set: (name) => {
    const nextName = String(name || '').trim()
    if (!nextName) return
    const s = mergeSignalSettings(signalSettingsState.value)
    const idx = s.screenStrategies.findIndex((item) => item.id === s.activeScreenStrategyId)
    if (idx >= 0) {
      s.screenStrategies[idx] = { ...s.screenStrategies[idx], name: nextName }
      signalSettingsState.value = s
    }
  },
})

function createStrategyId() {
  return `strategy-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 7)}`
}

function addScreenStrategy() {
  const s = mergeSignalSettings(signalSettingsState.value)
  const current = s.screenStrategies.find((item) => item.id === s.activeScreenStrategyId) || s.screenStrategies[0]
  const item = {
    id: createStrategyId(),
    name: `参数预设 ${s.screenStrategies.length + 1}`,
    settings: extractSignalStrategySettings(current?.settings || s),
  }
  s.screenStrategies = [...s.screenStrategies, item]
  s.activeScreenStrategyId = item.id
  signalSettingsState.value = s
}

function duplicateScreenStrategy() {
  const s = mergeSignalSettings(signalSettingsState.value)
  const current = s.screenStrategies.find((item) => item.id === s.activeScreenStrategyId) || s.screenStrategies[0]
  const item = {
    id: createStrategyId(),
    name: `${current.name} 副本`,
    settings: extractSignalStrategySettings(current.settings),
  }
  s.screenStrategies = [...s.screenStrategies, item]
  s.activeScreenStrategyId = item.id
  signalSettingsState.value = s
}

function deleteScreenStrategy() {
  const s = mergeSignalSettings(signalSettingsState.value)
  if (s.screenStrategies.length <= 1) {
    message.warning('至少保留一个参数预设')
    return
  }
  const idx = s.screenStrategies.findIndex((item) => item.id === s.activeScreenStrategyId)
  s.screenStrategies = s.screenStrategies.filter((item) => item.id !== s.activeScreenStrategyId)
  s.activeScreenStrategyId = s.screenStrategies[Math.max(0, idx - 1)]?.id || s.screenStrategies[0]?.id || DEFAULT_SCREEN_STRATEGY_ID
  signalSettingsState.value = s
}

const formValue = ref({
  ID: 1,
  tushareToken: '',
  dingPush: {
    enable: false,
    dingRobot: ''
  },
  localPush: {
    enable: true,
  },
  updateBasicInfoOnStart: false,
  refreshInterval: 1,
  openAI: {
    enable: false,
    aiConfigs: [], // AI配置列表
    prompt: "",
    questionTemplate: "{{stockName}}分析和总结",
    crawlTimeOut: 30,
    kDays: 30,
    httpProxy:"",
    httpProxyEnabled:false,
  },
  enableDanmu: false,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enablePushNews: true,
  enableOnlyPushRedNews: true,
  sponsorCode: "",
  httpProxy:"",
  httpProxyEnabled:false,
  enableAgent: false,
  qgqpBId: '',
})

function normalizeQgqpBId(value) {
  return String(value ?? '').trim()
}

// 添加一个新的AI配置到列表
function addAiConfig() {
  formValue.value.openAI.aiConfigs.push(new data.AIConfig({
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-reasoner',
    temperature: 0.1,
    maxTokens: 8192,
    timeOut: 6000,
    httpProxy:"",
    httpProxyEnabled:false,
  }));
}

// 从列表中移除一个AI配置
function removeAiConfig(index) {
  const originalCount = formValue.value.openAI.aiConfigs.length;
  // 使用filter创建新数组确保响应式更新
  formValue.value.openAI.aiConfigs = formValue.value.openAI.aiConfigs.filter((_, i) => i !== index);
}

// 根据接口地址与 apiKey 自动获取模型列表，并填充到当前 aiConfig
async function fetchAiModels(aiConfig) {
  if (!aiConfig.baseUrl || !aiConfig.apiKey) {
    message.warning('请先填写接口地址和 apiKey')
    return
  }
  if (aiConfig._loadingModels) {
    return
  }
  aiConfig._loadingModels = true
  try {
    const list = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey)
    const options = (list || []).map(id => ({ label: id, value: id }))
    aiConfig._modelOptions = options
    if (!aiConfig.modelName && options.length > 0) {
      aiConfig.modelName = options[0].value
    }
    if (!options.length) {
      message.warning('未从接口获取到可用模型，请检查地址和 apiKey')
    }
  } catch (e) {
    console.error('FetchAiModels error', e)
    message.error('获取模型列表失败，请检查接口地址和 apiKey')
  } finally {
    aiConfig._loadingModels = false
  }
}


const promptTemplates = ref([])

const aiPlatformOptions = [
  { label: 'DeepSeek (https://api.deepseek.com)', value: 'https://api.deepseek.com' },
  { label: '硅基流动 (https://api.siliconflow.cn/v1)', value: 'https://api.siliconflow.cn/v1' },
  { label: '智谱AI(GLM) (https://open.bigmodel.cn/api/paas/v4)', value: 'https://open.bigmodel.cn/api/paas/v4' },
  { label: '字节豆包(火山引擎) (https://ark.cn-beijing.volces.com/api/v3)', value: 'https://ark.cn-beijing.volces.com/api/v3' },
  { label: '阿里云百炼 (https://dashscope.aliyuncs.com/compatible-mode/v1)', value: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { label: 'Moonshot(月之暗面) (https://api.moonshot.cn/v1)', value: 'https://api.moonshot.cn/v1' },
  { label: '腾讯混元 (https://api.hunyuan.cloud.tencent.com/v1)', value: 'https://api.hunyuan.cloud.tencent.com/v1' },
  { label: '讯飞星火 (https://spark-api-open.xf-yun.com/v1)', value: 'https://spark-api-open.xf-yun.com/v1' },
  { label: '零一万物 (https://api.lingyiwanwu.com/v1)', value: 'https://api.lingyiwanwu.com/v1' },
  { label: 'MiniMax (https://api.minimax.chat/v1)', value: 'https://api.minimax.chat/v1' },
  { label: '百川智能 (https://api.baichuan-ai.com/v1)', value: 'https://api.baichuan-ai.com/v1' },
  { label: '百度千帆 (https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop)', value: 'https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop' },
  { label: 'OpenAI (https://api.openai.com/v1)', value: 'https://api.openai.com/v1' },
  { label: 'Azure OpenAI (https://YOUR_RESOURCE.openai.azure.com)', value: 'https://YOUR_RESOURCE.openai.azure.com' },
  { label: 'OpenRouter (https://openrouter.ai/api/v1)', value: 'https://openrouter.ai/api/v1' },
  { label:'Ollama (http://localhost:11434/v1)', value: 'http://localhost:11434/v1' },
]

function getPlatformName(baseUrl) {
  if (!baseUrl) return ''
  const platform = aiPlatformOptions.find(opt => opt.value === baseUrl)
  if (platform) {
    const idx = platform.label.indexOf(' (')
    return idx > 0 ? platform.label.substring(0, idx) : platform.label
  }
  return ''
}

function onBaseUrlChange(aiConfig, newBaseUrl) {
  const platformName = getPlatformName(newBaseUrl)
  if (platformName && aiConfig.name && !aiConfig.name.startsWith(platformName)) {
    aiConfig.name = platformName + '-' + aiConfig.name
  } else if (platformName && !aiConfig.name) {
    aiConfig.name = platformName
  }
}

function onModelNameChange(aiConfig, newModelName) {
  if (!newModelName) return
  const platformName = getPlatformName(aiConfig.baseUrl)
  const baseName = platformName || 'AI'
  
  if (!aiConfig.name) {
    aiConfig.name = baseName + '-' + newModelName
  } else if (aiConfig.name === platformName) {
    aiConfig.name = platformName + '-' + newModelName
  } else {
    const parts = aiConfig.name.split('-')
    if (parts.length >= 2 && parts[0] === platformName) {
      parts[parts.length - 1] = newModelName
      aiConfig.name = parts.join('-')
    } else if (!aiConfig.name.endsWith(newModelName)) {
      aiConfig.name = aiConfig.name + '-' + newModelName
    }
  }
}

onMounted(() => {
  GetConfig().then(res => {
    formValue.value.ID = res.ID
    formValue.value.tushareToken = res.tushareToken
    formValue.value.dingPush = {
      enable: res.dingPushEnable,
      dingRobot: res.dingRobot
    }
    formValue.value.localPush = {
      enable: res.localPushEnable,
    }
    formValue.value.updateBasicInfoOnStart = res.updateBasicInfoOnStart
    formValue.value.refreshInterval = res.refreshInterval
    // 加载AI配置
    formValue.value.openAI = {
      enable: res.openAiEnable,
      aiConfigs: res.aiConfigs || [],
      prompt: res.prompt,
      questionTemplate: res.questionTemplate ? res.questionTemplate : '{{stockName}}分析和总结',
      crawlTimeOut: res.crawlTimeOut,
      kDays: res.kDays,
      httpProxy:"",
      httpProxyEnabled:false,
    }


    formValue.value.enableDanmu = res.enableDanmu
    formValue.value.browserPath = res.browserPath
    formValue.value.enableNews = res.enableNews
    formValue.value.darkTheme = res.darkTheme
    formValue.value.enableFund = res.enableFund
    formValue.value.enablePushNews = res.enablePushNews
    formValue.value.enableOnlyPushRedNews = res.enableOnlyPushRedNews
    formValue.value.sponsorCode = res.sponsorCode
    formValue.value.httpProxy=res.httpProxy;
    formValue.value.httpProxyEnabled=res.httpProxyEnabled;
    formValue.value.enableAgent = res.enableAgent;
    formValue.value.qgqpBId = normalizeQgqpBId(res.qgqpBId)
    applySignalParamsFromConfig(res.signalParams)

  })

  // GetPromptTemplates("", "").then(res => {
  //   promptTemplates.value = res
  // })
})
onBeforeUnmount(() => {
  message.destroyAll()
})

function saveConfig() {
  console.log('开始保存设置', formValue.value);
  signalSettingsState.value = mergeSignalSettings(signalSettingsState.value)
  // 构建配置时，包含aiConfigs列表
  let config = new data.SettingConfig({
    ID: formValue.value.ID,
    dingPushEnable: formValue.value.dingPush.enable,
    dingRobot: formValue.value.dingPush.dingRobot,
    localPushEnable: formValue.value.localPush.enable,
    updateBasicInfoOnStart: formValue.value.updateBasicInfoOnStart,
    refreshInterval: formValue.value.refreshInterval,
    openAiEnable: formValue.value.openAI.enable,
    aiConfigs: formValue.value.openAI.aiConfigs,
    // 序列化aiConfigs列表以传递给后端
    tushareToken: formValue.value.tushareToken,
    prompt: formValue.value.openAI.prompt,
    questionTemplate: formValue.value.openAI.questionTemplate,
    crawlTimeOut: formValue.value.openAI.crawlTimeOut,
    kDays: formValue.value.openAI.kDays,
    enableDanmu: formValue.value.enableDanmu,
    browserPath: formValue.value.browserPath,
    enableNews: formValue.value.enableNews,
    darkTheme: formValue.value.darkTheme,
    enableFund: formValue.value.enableFund,
    enablePushNews: formValue.value.enablePushNews,
    enableOnlyPushRedNews: formValue.value.enableOnlyPushRedNews,
    sponsorCode: formValue.value.sponsorCode,
    httpProxy:formValue.value.httpProxy,
    httpProxyEnabled:formValue.value.httpProxyEnabled,
    enableAgent: formValue.value.enableAgent,
    qgqpBId: normalizeQgqpBId(formValue.value.qgqpBId),
    signalParams: serializeSignalParams(signalSettingsState.value),
  })

  UpdateConfig(config).then(res => {
    message.success(res)
    EventsEmit("updateSettings", config);
  })
}


function getHeight() {
  return document.documentElement.clientHeight
}

function sendTestNotice() {
  let markdown = "### go-stock test\n" + new Date()
  let msg = '{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"go-stock' + new Date() + '",' +
      '         "text": "' + markdown + '"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'

  SendDingDingMessageByType(msg, "test-" + new Date().getTime(), 1).then(res => {
    message.info(res)
  })
}

function exportConfig() {
  ExportConfig().then(res => {
    message.info(res)
  })
}

function importConfig() {
  let input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    let file = e.target.files[0];
    let reader = new FileReader();
    reader.onload = (e) => {
      let config = JSON.parse(e.target.result);
      formValue.value.ID = config.ID
      formValue.value.tushareToken = config.tushareToken
      formValue.value.dingPush = {
        enable: config.dingPushEnable,
        dingRobot: config.dingRobot
      }
      formValue.value.localPush = {
        enable: config.localPushEnable,
      }
      formValue.value.updateBasicInfoOnStart = config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval
      // 导入AI配置
      formValue.value.openAI = {
        enable: config.openAiEnable,
        aiConfigs: config.aiConfigs || [],
        prompt: config.prompt,
        questionTemplate: config.questionTemplate,
        crawlTimeOut: config.crawlTimeOut,
        kDays: config.kDays
      }
      formValue.value.enableDanmu = config.enableDanmu
      formValue.value.browserPath = config.browserPath
      formValue.value.enableNews = config.enableNews
      formValue.value.darkTheme = config.darkTheme
      formValue.value.enableFund = config.enableFund
      formValue.value.enablePushNews = config.enablePushNews
      formValue.value.enableOnlyPushRedNews = config.enableOnlyPushRedNews
      formValue.value.sponsorCode = config.sponsorCode
      formValue.value.httpProxy=config.httpProxy
      formValue.value.httpProxyEnabled=config.httpProxyEnabled
      formValue.value.enableAgent = config.enableAgent
      formValue.value.qgqpBId = normalizeQgqpBId(config.qgqpBId)
      applySignalParamsFromConfig(config.signalParams)
    };
    reader.readAsText(file);
  };
  input.click();
}


window.onerror = function (event, source, lineno, colno, error) {
  EventsEmit("frontendError", {
    page: "settings.vue",
    message: event,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null
  });
  return true;
};

const showManagePromptsModal = ref(false)
const promptTypeOptions = [
  {label: "模型系统Prompt", value: '模型系统Prompt'},
  {label: "模型用户Prompt", value: '模型用户Prompt'},]
const formPromptRef = ref(null)
const formPrompt = ref({
  ID: 0,
  Name: '',
  Content: '',
  Type: '',
})

function managePrompts() {
  formPrompt.value.ID = 0
  showManagePromptsModal.value = true
}

function savePrompt() {
  AddPrompt(formPrompt.value).then(res => {
    message.success(res)
    GetPromptTemplates("", "").then(res => {
      promptTemplates.value = res
    })
    showManagePromptsModal.value = false
  })
}

function editPrompt(prompt) {
  formPrompt.value.ID = prompt.ID
  formPrompt.value.Name = prompt.name
  formPrompt.value.Content = prompt.content
  formPrompt.value.Type = prompt.type
  showManagePromptsModal.value = true
}

function deletePrompt(ID) {
  DelPrompt(ID).then(res => {
    message.success(res)
    GetPromptTemplates("", "").then(res => {
      promptTemplates.value = res
    })
  })
}
</script>

<template>
  <n-flex justify="left" style="text-align: left; --wails-draggable:no-drag">
    <n-form ref="formRef" :label-placement="'left'" :label-align="'left'">
      <n-space vertical size="large">
        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '基础设置')" size="small">
          <n-grid :cols="24" :x-gap="16" :y-gap="8" style="text-align: left">
            <n-form-item-gi :span="8" label="Tushare Token" path="tushareToken">
              <n-input
                type="password"
                show-password-on="click"
                placeholder="tushare.pro 注册后获取"
                v-model:value="formValue.tushareToken"
                clearable
              />
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="启动更新基础信息" path="updateBasicInfoOnStart">
              <n-space align="center" :size="6">
                <n-switch v-model:value="formValue.updateBasicInfoOnStart"/>
                <n-tooltip placement="top">
                  <template #trigger>
                    <n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon>
                  </template>
                  开启后启动时从 Tushare 拉取 A 股/指数基础名单；须先填写 Token。
                </n-tooltip>
              </n-space>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="刷新间隔" path="refreshInterval">
              <n-input-number v-model:value="formValue.refreshInterval" placeholder="秒" style="width: 100%">
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item-gi>
            <n-form-item-gi :span="2" label="暗黑" path="darkTheme">
              <n-switch v-model:value="formValue.darkTheme"/>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="东财 qgqp_b_id" path="qgqpBId">
              <n-input
                type="text"
                placeholder="选填"
                autocomplete="off"
                v-model:value="formValue.qgqpBId"
                clearable
              >
                <template #suffix>
                  <n-tooltip placement="top">
                    <template #trigger>
                      <n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon>
                    </template>
                    F12 网络面板 → 任请求 Cookie → 复制 qgqp_b_id
                  </n-tooltip>
                </template>
              </n-input>
            </n-form-item-gi>
            <n-form-item-gi :span="24" label="浏览器路径" path="browserPath">
              <n-input type="text" placeholder="Edge / Chrome 可执行文件路径" v-model:value="formValue.browserPath" clearable/>
            </n-form-item-gi>
          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '通知设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="3" label="钉钉推送：" path="dingPush.enable">
              <n-switch v-model:value="formValue.dingPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="本地推送：" path="localPush.enable">
              <n-switch v-model:value="formValue.localPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="弹幕功能：" path="enableDanmu">
              <n-switch v-model:value="formValue.enableDanmu"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="显示滚动快讯：" path="enableNews">
              <n-switch v-model:value="formValue.enableNews"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="市场资讯提醒：" path="enablePushNews">
              <n-switch v-model:value="formValue.enablePushNews"/>
            </n-form-item-gi>
            <n-form-item-gi v-if="formValue.enablePushNews" :span="4" label="只提醒红字或关注个股的新闻：" path="enableOnlyPushRedNews">
              <n-switch v-model:value="formValue.enableOnlyPushRedNews"/>
            </n-form-item-gi>
            <n-form-item-gi :span="22" v-if="formValue.dingPush.enable" label="钉钉机器人接口地址："
                            path="dingPush.dingRobot">
              <n-input placeholder="请输入钉钉机器人接口地址" v-model:value="formValue.dingPush.dingRobot"/>
              <n-button type="primary" @click="sendTestNotice">发送测试通知</n-button>
            </n-form-item-gi>
          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'AI设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left;">
            <n-form-item-gi :span="24" label="AI诊股：" path="openAI.enable">
              <n-switch v-model:value="formValue.openAI.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="6" v-if="formValue.openAI.enable" label="Crawler Timeout(秒)"
                            title="资讯采集超时时间(秒)" path="openAI.crawlTimeOut">
              <n-input-number min="30" step="1" v-model:value="formValue.openAI.crawlTimeOut"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" v-if="formValue.openAI.enable" title="天数越多消耗tokens越多"
                            label="日K线数据(天)" path="openAI.kDays">
              <n-input-number min="30" step="1" max="60" v-model:value="formValue.openAI.kDays"/>
            </n-form-item-gi>
            <n-form-item-gi :span="2" label="爬虫http代理" path="httpProxyEnabled">
              <n-switch v-model:value="formValue.httpProxyEnabled"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" v-if="formValue.httpProxyEnabled" title="http代理地址"
                            label="http代理地址" path="httpProxy">
              <n-input type="text" placeholder="爬虫http代理地址" v-model:value="formValue.httpProxy" clearable/>
            </n-form-item-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">默认提示词设置</n-divider>
            </n-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="默认系统提示词" path="openAI.prompt">
              <n-input v-model:value="formValue.openAI.prompt" type="textarea" :show-count="true"
                       placeholder="请输入系统提示词" :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>
            <n-form-item-gi :span="12" v-if="formValue.openAI.enable" label="默认个股分析提示词"
                            path="openAI.questionTemplate">
              <n-input v-model:value="formValue.openAI.questionTemplate" type="textarea" :show-count="true"
                       placeholder="请输入个股分析提示词:例如{{stockName}}[{{stockCode}}]分析和总结"
                       :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-divider title-placement="left">AI模型服务配置</n-divider>
            </n-gi>
            <n-gi :span="24" v-if="formValue.openAI.enable">
              <n-space vertical>
                <n-card v-for="(aiConfig, index) in formValue.openAI.aiConfigs" :key="index" :bordered="true"
                        size="small">
                  <template #header>
                    <n-flex justify="space-between" align="center">
                      <n-text depth="3">AI 配置 #{{ index + 1 }}</n-text>
                      <n-button type="error" size="tiny" ghost @click="removeAiConfig(index)">删除</n-button>
                    </n-flex>
                  </template>
                  <n-grid :cols="24" :x-gap="24">
                    <n-form-item-gi :span="24" hidden label="配置ID" :path="`openAI.aiConfigs[${index}].ID`">
                      <n-input type="text" placeholder="配置ID" v-model:value="aiConfig.ID" clearable/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="配置名称" :path="`openAI.aiConfigs[${index}].name`">
                      <n-input type="text" placeholder="配置名称" v-model:value="aiConfig.name" clearable/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="接口地址" :path="`openAI.aiConfigs[${index}].baseUrl`">
                      <n-select
                        v-model:value="aiConfig.baseUrl"
                        :options="aiPlatformOptions"
                        filterable
                        tag
                        clearable
                        placeholder="选择或输入AI接口地址"
                        @update:value="(val) => onBaseUrlChange(aiConfig, val)"
                      />
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="令牌(apiKey)" :path="`openAI.aiConfigs[${index}].apiKey`">
                      <n-input type="password" placeholder="apiKey" v-model:value="aiConfig.apiKey" clearable
                               show-password-on="click"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="8" label="模型名称" :path="`openAI.aiConfigs[${index}].modelName`">
                      <n-select
                        v-model:value="aiConfig.modelName"
                        :options="aiConfig._modelOptions || []"
                        filterable
                        tag
                        :loading="aiConfig._loadingModels"
                        placeholder="点击获取模型列表或手动输入"
                        @click="fetchAiModels(aiConfig)"
                        @update:value="(val) => onModelNameChange(aiConfig, val)"
                      />
                    </n-form-item-gi>
                    <n-form-item-gi :span="5" label="Temperature" :path="`openAI.aiConfigs[${index}].temperature`">
                      <n-input-number placeholder="temperature" v-model:value="aiConfig.temperature" :step="0.1"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="5" label="MaxTokens" :path="`openAI.aiConfigs[${index}].maxTokens`">
                      <n-input-number placeholder="maxTokens" v-model:value="aiConfig.maxTokens"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="5" label="Timeout(秒)" :path="`openAI.aiConfigs[${index}].timeOut`">
                      <n-input-number min="60" step="1" placeholder="超时(秒)" v-model:value="aiConfig.timeOut"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" label="http代理" :path="`openAI.aiConfigs[${index}].httpProxyEnabled`">
                      <n-switch v-model:value="aiConfig.httpProxyEnabled"/>
                    </n-form-item-gi>
                    <n-form-item-gi :span="12" v-if="aiConfig.httpProxyEnabled" title="http代理地址" :path="`openAI.aiConfigs[${index}].httpProxy`">
                      <n-input type="text" placeholder="http代理地址" v-model:value="aiConfig.httpProxy" clearable/>
                    </n-form-item-gi>
                  </n-grid>
                </n-card>
                <n-button type="primary" dashed @click="addAiConfig" style="width: 100%;">+ 添加AI配置</n-button>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-card size="small">
                <template #header>
                  <n-space align="center" :size="8">
                    <n-tag type="warning" :bordered="false">K线信号参数</n-tag>
                    <n-text depth="3" style="font-size: 12px">
                      自定义买卖点识别阈值 · 只影响未来扫描，不改历史计划与策略解释
                    </n-text>
                    <n-tag size="small" type="info" :bordered="false">Beta</n-tag>
                  </n-space>
                </template>
                <div class="screen-strategy-manager">
                  <n-space align="center" :size="[8, 8]" wrap>
                    <n-text depth="3">当前参数预设</n-text>
                    <n-select
                      v-model:value="activeStrategyId"
                      :options="strategyOptions"
                      style="width: 180px"
                    />
                    <n-input
                      v-model:value="activeStrategyName"
                      placeholder="参数预设名称"
                      style="width: 180px"
                    />
                    <n-button size="small" tertiary type="primary" @click="addScreenStrategy">新建预设</n-button>
                    <n-button size="small" tertiary @click="duplicateScreenStrategy">复制</n-button>
                    <n-button size="small" tertiary type="error" @click="deleteScreenStrategy">删除</n-button>
                    <n-text depth="3" style="font-size: 12px">信号参数预设（非交易 Strategy）；股票筛选页可按预设筛选或生成快照</n-text>
                  </n-space>
                </div>
                <SignalSettingsPanel v-model="activeStrategySettings" :show-display="false" />
              </n-card>
            </n-gi>

            <n-gi :span="24">
              <n-card size="small">
                <template #header>
                  <n-space align="center" :size="8">
                    <n-tag type="info" :bordered="false">自选卡片</n-tag>
                    <n-text depth="3" style="font-size: 12px">全局界面显示设置，不随选股策略切换</n-text>
                  </n-space>
                </template>
                <SignalSettingsPanel v-model="globalDisplaySettings" :show-signal-rules="false" />
              </n-card>
            </n-gi>

            <n-gi :span="24">
              <n-card size="small">
                <template #header>
                  <n-space align="center" :size="8">
                    <n-tag type="success" :bordered="false">量化自动化</n-tag>
                    <n-text depth="3" style="font-size: 12px">信号提醒 · 区间触达 · 买入清单 · 仓位计算</n-text>
                  </n-space>
                </template>
                <QuantAutomationPanel v-model="signalSettingsState" />
              </n-card>
            </n-gi>

            <n-gi :span="24">
              <n-divider/>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-space justify="center">
                  <n-button type="primary" strong @click="saveConfig">保存设置</n-button>
                  <n-button type="info" @click="exportConfig">导出配置</n-button>
                  <n-button type="error" @click="importConfig">导入配置</n-button>
                </n-space>
                <n-flex justify="start" style="margin-top: 10px" v-if="promptTemplates.length > 0">
                  <n-tag :bordered="false" type="warning">提示词模板:</n-tag>
                  <n-tag size="medium" secondary v-for="prompt in promptTemplates" closable
                         @close="deletePrompt(prompt.ID)" @click="editPrompt(prompt)" :title="prompt.content"
                         :type="prompt.type === '模型系统Prompt' ? 'success' : 'info'" :bordered="false">{{
                      prompt.name
                    }}
                  </n-tag>
                </n-flex>
              </n-space>
            </n-gi>
          </n-grid>
        </n-card>
      </n-space>
    </n-form>
  </n-flex>

  <n-modal v-model:show="showManagePromptsModal" closable :mask-closable="false">
    <n-card style="width: 800px; height: 600px; text-align: left" :bordered="false"
            :title="(formPrompt.ID > 0 ? '修改' : '添加') + '提示词'" size="huge" role="dialog" aria-modal="true">
      <n-form ref="formPromptRef" :label-placement="'left'" :label-align="'left'">
        <n-form-item label="名称">
          <n-input v-model:value="formPrompt.Name" placeholder="请输入提示词名称"/>
        </n-form-item>
        <n-form-item label="类型">
          <n-select v-model:value="formPrompt.Type" :options="promptTypeOptions" placeholder="请选择提示词类型"/>
        </n-form-item>
        <n-form-item label="内容">
          <n-input v-model:value="formPrompt.Content" type="textarea" :show-count="true" placeholder="请输入prompt"
                   :autosize="{ minRows: 12, maxRows: 12, }"/>
        </n-form-item>
      </n-form>
      <template #footer>
        <n-flex justify="end">
          <n-button type="primary" @click="savePrompt">保存</n-button>
          <n-button type="warning" @click="showManagePromptsModal = false">取消</n-button>
        </n-flex>
      </template>
    </n-card>
  </n-modal>
</template>

<style scoped>
.cardHeaderClass {
  font-size: 16px;
  font-weight: bold;
  color: red;
}
.screen-strategy-manager {
  margin-bottom: 12px;
  padding: 10px 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  background: var(--n-action-color);
}
</style>
