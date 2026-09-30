<script setup>
/**
 * 实盘镜像（观察）手工录入。
 * source=external_mirror。不写 paper_sim，不进 TradePlan，不下单。
 */
import { onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSpace,
  NSpin,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import {
  createExternalMirrorHolding,
  deleteExternalMirrorHolding,
  listExternalMirrorHoldings,
  updateExternalMirrorHolding,
} from '../api/externalMirror'
import {
  COPY_TO_PAPER_STUB,
  EXTERNAL_MIRROR_DISCLAIMER,
  EXTERNAL_MIRROR_SOURCE,
  todayISODate,
  validateMirrorDraft,
} from '../utils/externalMirrorEntry.js'

const message = useMessage()
const dialog = useDialog()
const loading = ref(false)
const saving = ref(false)
const rows = ref([])
const disclaimer = ref(EXTERNAL_MIRROR_DISCLAIMER)
const editorOpen = ref(false)
const editingId = ref(0)

const form = reactive({
  stockCode: '',
  stockName: '',
  quantity: null,
  costPrice: null,
  entryDate: todayISODate(),
  note: '',
})

function resetForm() {
  form.stockCode = ''
  form.stockName = ''
  form.quantity = null
  form.costPrice = null
  form.entryDate = todayISODate()
  form.note = ''
  editingId.value = 0
}

function openCreate() {
  resetForm()
  editorOpen.value = true
}

function openEdit(row) {
  editingId.value = row.id
  form.stockCode = row.stockCode
  form.stockName = row.stockName || ''
  form.quantity = row.quantity
  form.costPrice = row.costPrice
  form.entryDate = row.entryDate || todayISODate()
  form.note = row.note || ''
  editorOpen.value = true
}

async function refresh() {
  loading.value = true
  try {
    const data = await listExternalMirrorHoldings()
    rows.value = data.holdings
    disclaimer.value = data.disclaimer || EXTERNAL_MIRROR_DISCLAIMER
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    loading.value = false
  }
}

async function save() {
  const draft = {
    stockCode: form.stockCode,
    stockName: form.stockName,
    quantity: form.quantity,
    costPrice: form.costPrice,
    entryDate: form.entryDate,
    note: form.note,
    source: EXTERNAL_MIRROR_SOURCE,
    feedsTradePlan: false,
    tradable: false,
  }
  const check = validateMirrorDraft(draft)
  if (!check.ok) {
    message.warning(check.message)
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateExternalMirrorHolding(editingId.value, draft)
      message.success('已更新镜像持仓')
    } else {
      await createExternalMirrorHolding(draft)
      message.success('已录入镜像持仓')
    }
    editorOpen.value = false
    resetForm()
    await refresh()
  } catch (e) {
    message.error(e?.message || String(e))
  } finally {
    saving.value = false
  }
}

function confirmDelete(row) {
  dialog.warning({
    title: '删除实盘镜像持仓',
    content: `确认删除 ${row.stockName || row.stockCode}（${row.stockCode}）？只删除观察记录，不会改模拟账本，也不会下单。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await deleteExternalMirrorHolding(row.id)
        message.success('已删除')
        await refresh()
      } catch (e) {
        message.error(e?.message || String(e))
      }
    },
  })
}

onMounted(refresh)
</script>

<template>
  <section class="mirror-panel">
    <n-alert type="warning" :bordered="false" style="margin-bottom: 12px">
      {{ disclaimer }}
    </n-alert>
    <n-space justify="space-between" align="center" style="margin-bottom: 10px">
      <n-space align="center" :size="8">
        <n-tag size="small" type="warning" :bordered="false">{{ EXTERNAL_MIRROR_SOURCE }}</n-tag>
        <n-text depth="3" style="font-size: 12px">
          手工对照真实券商持仓。自选股上的成本数量不会自动变成镜像。
        </n-text>
      </n-space>
      <n-space>
        <n-button
          size="small"
          disabled
          :title="'后续：显式复制为 paper_sim 影子仓，不会直接改镜像。'"
        >
          {{ COPY_TO_PAPER_STUB }}
        </n-button>
        <n-button size="small" secondary :loading="loading" @click="refresh">刷新</n-button>
        <n-button size="small" type="primary" @click="openCreate">新增</n-button>
      </n-space>
    </n-space>

    <n-spin :show="loading">
      <div class="table-wrap">
        <table v-if="rows.length" class="mirror-table">
          <thead>
            <tr>
              <th>代码</th>
              <th>名称</th>
              <th>数量</th>
              <th>成本价</th>
              <th>录入日</th>
              <th>备注</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in rows" :key="row.id">
              <td class="code">{{ row.stockCode }}</td>
              <td>{{ row.stockName || '—' }}</td>
              <td>{{ row.quantity }}</td>
              <td>{{ row.costPrice }}</td>
              <td>{{ row.entryDate }}</td>
              <td class="note">{{ row.note || '—' }}</td>
              <td>
                <n-button size="tiny" secondary @click="openEdit(row)">编辑</n-button>
                <n-button size="tiny" type="error" secondary style="margin-left: 6px" @click="confirmDelete(row)">
                  删除
                </n-button>
              </td>
            </tr>
          </tbody>
        </table>
        <n-empty v-else description="暂无实盘镜像持仓" />
      </div>
    </n-spin>

    <div v-if="editorOpen" class="editor">
      <n-text strong>{{ editingId ? '编辑镜像持仓' : '新增镜像持仓' }}</n-text>
      <n-text depth="3" style="display: block; margin: 4px 0 8px; font-size: 12px">
        缺代码、数量或成本价将拒绝保存。保存后仍只出现在本页。
      </n-text>
      <n-form label-placement="left" label-width="88">
        <n-form-item label="代码" required>
          <n-input
            v-model:value="form.stockCode"
            :disabled="!!editingId"
            placeholder="如 600519 / sz000001"
          />
        </n-form-item>
        <n-form-item label="名称">
          <n-input v-model:value="form.stockName" placeholder="可空，保存时尝试补全" />
        </n-form-item>
        <n-form-item label="数量" required>
          <n-input-number v-model:value="form.quantity" :min="1" :step="100" style="width: 100%" placeholder="大于 0" />
        </n-form-item>
        <n-form-item label="成本价" required>
          <n-input-number v-model:value="form.costPrice" :min="0.0001" :step="0.01" style="width: 100%" placeholder="大于 0" />
        </n-form-item>
        <n-form-item label="录入日">
          <n-input v-model:value="form.entryDate" placeholder="YYYY-MM-DD，默认为今天" />
        </n-form-item>
        <n-form-item label="备注">
          <n-input v-model:value="form.note" type="textarea" placeholder="可选" :autosize="{ minRows: 2, maxRows: 4 }" />
        </n-form-item>
      </n-form>
      <n-space justify="end">
        <n-button @click="editorOpen = false">取消</n-button>
        <n-button type="primary" :loading="saving" @click="save">保存</n-button>
      </n-space>
    </div>
  </section>
</template>

<style scoped>
.mirror-panel { margin-top: 4px; }
.table-wrap { overflow-x: auto; min-height: 120px; }
.mirror-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.mirror-table th, .mirror-table td { padding: 8px 10px; border-bottom: 1px solid rgba(128,128,128,.16); text-align: left; white-space: nowrap; }
.mirror-table th { color: #8a8f99; font-weight: 500; }
.code { font-family: Consolas, monospace; }
.note { max-width: 220px; overflow: hidden; text-overflow: ellipsis; }
.editor {
  margin-top: 14px;
  padding: 12px;
  border: 1px solid rgba(128,128,128,.2);
  border-radius: 8px;
}
</style>
