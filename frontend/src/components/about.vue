<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import {
  CheckLocalUpdate,
  GetVersionInfo,
  GetReleaseIdentity,
  ExportDiagnosticBundle,
  ExportDailyPilotReport,
} from '../../wailsjs/go/main/App'
import { NAlert, NButton, NTag, NText, useNotification } from 'naive-ui'

/** 联系方式占位（官方配置位，不含原开源作者信息） */
const CONTACT_PLACEHOLDER = {
  email: '',
  website: '',
  supportNote: '联系方式由官方配置后展示；当前为占位，未绑定个人作者或第三方收款渠道。',
}

const versionInfo = ref('')
const releaseIdentity = ref({
  version: '',
  build_time: '',
  git_commit: '',
  channel: '',
  build_mode: '',
})
const notify = useNotification()
const localUpdateChecking = ref(false)
const diagnosticExporting = ref(false)
const dailyPilotExporting = ref(false)

const updateStatus = ref('')
const updateCurrent = ref('')
const updateLatest = ref('')
const updateNote = ref('')
const updateURL = ref('')
const updateError = ref('')

const capabilities = [
  '智能选股',
  '组合决策',
  '风险管理',
  '模拟交易',
  '持仓管理',
  '再平衡分析',
]

const architecture = [
  { name: 'Go Backend', desc: '交易日流程、决策与风控服务' },
  { name: 'Wails Desktop', desc: '桌面壳与本地运行时' },
  { name: 'SQLite', desc: '本地持久化' },
  { name: 'AI Analysis Layer', desc: '研究与分析辅助层' },
]

const progressItems = [
  { phase: 'Phase12', title: 'Portfolio Architecture', desc: '组合决策、风险快照、Controlled / Shadow 旁路架构' },
  { phase: 'Phase13', title: 'Observation Layer', desc: '观察层、决策看板、Sell Suggestion、Coverage Audit' },
]

function applyUpdateResult(res) {
  const err = res?.error || res?.Error || ''
  updateCurrent.value = res?.current_version || res?.CurrentVersion || ''
  updateLatest.value = res?.latest_version || res?.LatestVersion || ''
  updateNote.value = res?.release_note || res?.ReleaseNote || ''
  updateURL.value = res?.download_url || res?.DownloadURL || ''
  updateError.value = err
  const st = res?.status || res?.Status || ''
  if (st) {
    updateStatus.value = st
  } else if (err) {
    updateStatus.value = 'Error'
  } else if (res?.update_available ?? res?.UpdateAvailable) {
    updateStatus.value = 'UpdateAvailable'
  } else {
    updateStatus.value = 'UpToDate'
  }
}

async function onCheckLocalUpdate(silent = false) {
  localUpdateChecking.value = true
  try {
    const res = await CheckLocalUpdate()
    applyUpdateResult(res)
    const err = updateError.value
    const available = updateStatus.value === 'UpdateAvailable'
    if (silent) return
    if (err) {
      notify.warning({ content: err })
      return
    }
    if (available) {
      notify.info({
        title: '发现新版本 ' + updateLatest.value,
        content: updateNote.value || '本地清单显示有新版本（不会自动下载安装）。',
        meta: updateURL.value ? '下载地址：' + updateURL.value : '请按官方发测渠道获取安装包',
        duration: 8000,
      })
    } else {
      notify.success({ content: '当前已是清单中的最新版本：' + updateCurrent.value })
    }
  } catch (e) {
    updateStatus.value = 'Error'
    updateError.value = String(e?.message || e || '检查失败')
    if (!silent) notify.error({ content: updateError.value })
  } finally {
    localUpdateChecking.value = false
  }
}

async function onExportDiagnostic() {
  diagnosticExporting.value = true
  try {
    const res = await ExportDiagnosticBundle()
    const text = String(res || '')
    if (text.startsWith('导出成功')) {
      notify.success({ content: text, duration: 5000 })
    } else if (text === '已取消') {
      notify.info({ content: '已取消导出' })
    } else {
      notify.warning({ content: text || '导出失败' })
    }
  } catch (e) {
    notify.error({ content: String(e?.message || e || '导出失败') })
  } finally {
    diagnosticExporting.value = false
  }
}

async function onExportDailyPilot() {
  dailyPilotExporting.value = true
  try {
    const res = await ExportDailyPilotReport()
    const text = String(res || '')
    if (text.startsWith('导出成功')) {
      if (text.includes('data_gaps')) {
        notify.warning({
          title: '已导出（含 data_gaps）',
          content: text,
          duration: 8000,
        })
      } else {
        notify.success({ content: text, duration: 5000 })
      }
    } else if (text === '已取消') {
      notify.info({ content: '已取消导出' })
    } else {
      notify.warning({ content: text || '导出失败' })
    }
  } catch (e) {
    notify.error({ content: String(e?.message || e || '导出失败') })
  } finally {
    dailyPilotExporting.value = false
  }
}

onMounted(() => {
  document.title = '关于 go-stock'
  GetReleaseIdentity()
    .then((id) => {
      if (!id) return
      releaseIdentity.value = {
        version: id.version || id.Version || '',
        build_time: id.build_time || id.BuildTime || '',
        git_commit: id.git_commit || id.GitCommit || id.commit_hash || id.CommitHash || '',
        channel: id.channel || id.Channel || '',
        build_mode: id.build_mode || id.BuildMode || '',
      }
    })
    .catch(() => {})
  onCheckLocalUpdate(true)
  GetVersionInfo()
    .then((res) => {
      versionInfo.value = res?.version || ''
    })
    .catch(() => {})
})

onBeforeUnmount(() => {
  notify.destroyAll()
})
</script>

<template>
  <n-space vertical size="large" style="--wails-draggable: no-drag">
    <n-card size="large">
      <n-divider title-placement="center">关于 go-stock</n-divider>

      <n-space vertical align="center">
        <h1>
          <n-badge :value="versionInfo || releaseIdentity.version || '—'" :offset="[80, 10]" type="success">
            <n-gradient-text type="info" :size="50">go-stock</n-gradient-text>
          </n-badge>
        </h1>
        <n-text depth="2" style="max-width: 640px; text-align: center">
          AI 驱动股票分析与组合决策平台
        </n-text>

        <n-alert type="info" :bordered="false" style="max-width: 560px; width: 100%; text-align: left">
          <n-text strong>go-stock Beta · 模拟研究模式</n-text>
          <n-text depth="3" style="display: block; margin-top: 6px; line-height: 1.6">
            当前版本面向朋友试用：股票研究、信号分析、策略计划、K 线分析。
            <br />
            不会自动交易，也不会连接券商真实下单。
          </n-text>
        </n-alert>

        <n-space>
          <n-button
            size="tiny"
            :loading="localUpdateChecking"
            type="primary"
            tertiary
            @click="() => onCheckLocalUpdate(false)"
          >
            检查更新
          </n-button>
          <n-button size="tiny" :loading="diagnosticExporting" type="warning" tertiary @click="onExportDiagnostic">
            导出诊断包
          </n-button>
          <n-button
            size="tiny"
            :loading="dailyPilotExporting"
            type="info"
            tertiary
            @click="onExportDailyPilot"
          >
            导出每日 Beta 观察报告
          </n-button>
        </n-space>

        <n-card size="small" embedded style="max-width: 560px; margin: 0 auto; text-align: left; width: 100%">
          <n-space vertical :size="6">
            <n-space align="center">
              <n-text strong>版本提醒</n-text>
              <n-tag v-if="updateStatus === 'UpToDate'" size="small" type="success" :bordered="false">UpToDate</n-tag>
              <n-tag
                v-else-if="updateStatus === 'UpdateAvailable'"
                size="small"
                type="warning"
                :bordered="false"
              >
                UpdateAvailable
              </n-tag>
              <n-tag v-else-if="updateStatus === 'Error'" size="small" type="error" :bordered="false">Error</n-tag>
              <n-tag v-else size="small" :bordered="false">未检查</n-tag>
            </n-space>
            <n-text depth="3" style="font-size: 12px">
              当前版本：{{ updateCurrent || releaseIdentity.version || versionInfo || '—' }}
            </n-text>
            <n-text depth="3" style="font-size: 12px">最新版本：{{ updateLatest || '—' }}</n-text>
            <n-text v-if="updateNote" depth="2" style="font-size: 12px; white-space: pre-wrap">
              更新说明：{{ updateNote }}
            </n-text>
            <n-text v-if="updateURL" depth="3" style="font-size: 11px">
              下载指引：{{ updateURL }}（需手动获取，不会自动下载安装）
            </n-text>
            <n-alert v-if="updateError" type="warning" :bordered="false" style="font-size: 12px">
              {{ updateError }}
            </n-alert>
            <n-text depth="3" style="font-size: 11px">读取本地 data/version.json；禁止自动下载 / 自动安装。</n-text>
          </n-space>
        </n-card>

        <n-text depth="3" style="font-size: 11px">
          诊断包为本地 zip（diag-2），含版本/任务/交易链摘要，不含持仓/成交/密码/API Key/现金/账户 ID，不会上传服务器。
        </n-text>
        <n-text depth="3" style="font-size: 11px">
          每日 Beta 观察报告为本地 JSON（DailyPilotReport）；只读聚合，不自动交易、不切换 Controlled；无观察源时 JSON 含
          data_gaps，不会上传云端。
        </n-text>
        <n-text depth="3" style="font-size: 12px; text-align: left; display: block; max-width: 640px">
          version：{{ releaseIdentity.version || versionInfo || '—' }} · build_time：{{ releaseIdentity.build_time || '—' }} ·
          git_commit：{{ releaseIdentity.git_commit || '—' }} · channel：{{ releaseIdentity.channel || '—' }} ·
          build_mode：{{ releaseIdentity.build_mode || '—' }}
        </n-text>
      </n-space>

      <!-- 1. 产品介绍 -->
      <n-divider title-placement="center">产品介绍</n-divider>
      <div class="section-body">
        <p>
          <strong>go-stock</strong> 是 AI 驱动的股票分析与组合决策平台，面向研究与投资分析辅助场景，提供选股、组合决策、风险观察与模拟交易等能力。
        </p>
      </div>

      <!-- 2. 核心能力 -->
      <n-divider title-placement="center">核心能力</n-divider>
      <n-flex justify="center" style="margin-bottom: 8px">
        <n-space :wrap="true" justify="center" style="max-width: 720px">
          <n-tag v-for="c in capabilities" :key="c" size="medium" type="info" :bordered="false">
            {{ c }}
          </n-tag>
        </n-space>
      </n-flex>

      <!-- 3. 技术架构 -->
      <n-divider title-placement="center">技术架构</n-divider>
      <n-flex justify="center">
        <n-table size="small" style="width: 560px; max-width: 100%">
          <n-thead>
            <n-tr>
              <n-th>组件</n-th>
              <n-th>说明</n-th>
            </n-tr>
          </n-thead>
          <n-tbody>
            <n-tr v-for="row in architecture" :key="row.name">
              <n-td>{{ row.name }}</n-td>
              <n-td>{{ row.desc }}</n-td>
            </n-tr>
          </n-tbody>
        </n-table>
      </n-flex>

      <!-- 4. 项目进展 -->
      <n-divider title-placement="center">项目进展</n-divider>
      <n-flex justify="center">
        <n-space vertical style="width: 560px; max-width: 100%; text-align: left">
          <n-card v-for="p in progressItems" :key="p.phase" size="small" embedded>
            <n-space align="center" style="margin-bottom: 4px">
              <n-tag size="small" type="success" :bordered="false">{{ p.phase }}</n-tag>
              <n-text strong>{{ p.title }}</n-text>
            </n-space>
            <n-text depth="3" style="font-size: 13px">{{ p.desc }}</n-text>
          </n-card>
        </n-space>
      </n-flex>

      <!-- 5. 免责声明 -->
      <n-divider title-placement="center">免责声明</n-divider>
      <div class="section-body">
        <n-alert type="warning" :bordered="false">
          本软件仅用于<strong>研究和投资分析辅助</strong>，<strong>不构成投资建议</strong>。
          市场有风险，决策与盈亏由使用者自行承担。本页不提供任何荐股、下单或自动交易承诺。
        </n-alert>
      </div>

      <!-- 6. 联系方式（占位） -->
      <n-divider title-placement="center">联系方式</n-divider>
      <div class="section-body">
        <p>{{ CONTACT_PLACEHOLDER.supportNote }}</p>
        <p>
          邮箱：
          <n-text depth="3">{{ CONTACT_PLACEHOLDER.email || '（待配置）' }}</n-text>
        </p>
        <p>
          官网 / 文档：
          <n-text depth="3">{{ CONTACT_PLACEHOLDER.website || '（待配置）' }}</n-text>
        </p>
      </div>
    </n-card>
  </n-space>
</template>

<style scoped>
h1,
h2 {
  margin: 0;
  padding: 6px 0;
}

p {
  margin: 6px 0;
  line-height: 1.6;
}

.section-body {
  justify-self: center;
  text-align: left;
  max-width: 640px;
  margin: 0 auto;
  padding: 0 8px 8px;
}

a {
  color: #18a058;
  text-decoration: none;
}

a:hover {
  text-decoration: underline;
}
</style>
