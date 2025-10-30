<template>
  <div class="space-y-6 flex flex-col h-full overflow-hidden">
    <div class="title-deco">
      <h2>SSH 终端</h2>
    </div>

    <div v-if="!currentServer" class="card p-8 text-center">
      <AlertCircleIcon class="size-12 text-muted-foreground mx-auto mb-4" />
      <p class="text-muted-foreground">请先在左侧选择一个服务器</p>
    </div>

    <div v-else-if="currentServer.status !== 'connected'" class="card p-8 text-center">
      <AlertCircleIcon class="size-12 text-muted-foreground mx-auto mb-4" />
      <p class="text-muted-foreground mb-4">服务器未连接</p>
      <button class="btn btn-sm btn-primary" @click="serverStore.connectServer(currentServer.id)">
        连接服务器
      </button>
    </div>

    <div v-else class="card p-4 flex-1 flex flex-col overflow-hidden">
      <div class="flex gap-2 mb-4 flex-wrap">
        <div class="flex items-center gap-2 px-3 py-2 bg-muted rounded-lg flex-1 min-w-[200px]">
          <ServerIcon class="size-4 text-primary" />
          <span class="text-sm">{{ currentServer.name }} ({{ currentServer.host }})</span>
        </div>
        <button
          class="btn btn-outline btn-sm"
          @click="showQuickCommands = !showQuickCommands"
          title="常用命令">
          <CommandIcon class="size-4" />
          <span class="hidden sm:inline">常用命令</span>
        </button>
        <button class="btn btn-outline btn-sm" @click="showHistory = !showHistory" title="历史记录">
          <HistoryIcon class="size-4" />
          <span class="hidden sm:inline">历史</span>
        </button>
        <button
          class="btn btn-outline btn-sm"
          @click="terminalStore.clearTerminal"
          title="清空终端">
          <Trash2Icon class="size-4" />
          <span class="hidden sm:inline">清空</span>
        </button>
        <label
          for="autoScroll"
          class="label select-none btn-sm btn btn-outline flex items-center cursor-pointer">
          自动滚动
          <input
            type="checkbox"
            id="autoScroll"
            class="checkbox checkbox-primary checkbox-md"
            v-model="autoScrollDown"
            title="自动滚动" />
        </label>
      </div>

      <!-- 常用命令面板 -->
      <div v-if="showQuickCommands" class="mb-4 p-4 bg-muted rounded-lg">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-semibold">常用命令</h3>
          <button class="btn btn-xs btn-primary" @click="showAddCommand = !showAddCommand">
            <PlusIcon class="size-3" />
            添加
          </button>
        </div>

        <!-- 添加命令表单 -->
        <div v-if="showAddCommand" class="mb-3 p-3 bg-background rounded border">
          <div class="space-y-2">
            <input
              v-model="newCommandName"
              placeholder="命令名称（如：查看进程）"
              class="w-full px-2 py-1 text-sm border rounded"
              @keyup.enter="handleAddQuickCommand" />
            <input
              v-model="newCommandValue"
              placeholder="命令内容（如：ps aux）"
              class="w-full px-2 py-1 text-sm border rounded"
              @keyup.enter="handleAddQuickCommand" />
            <div class="flex gap-2">
              <button class="btn btn-xs btn-primary" @click="handleAddQuickCommand">确定</button>
              <button class="btn btn-xs btn-outline" @click="showAddCommand = false">取消</button>
            </div>
          </div>
        </div>

        <!-- 命令列表 -->
        <div class="flex flex-wrap gap-2">
          <div
            v-for="(cmd, index) in quickCommands"
            :key="index"
            class="group relative inline-flex items-center gap-1 px-3 py-1.5 bg-background hover:bg-primary/10 border rounded-lg cursor-pointer transition-colors text-sm">
            <button @click="executeQuickCommand(cmd.command)" class="flex items-center gap-1.5">
              <TerminalIcon class="size-3" />
              <span>{{ cmd.name }}</span>
            </button>
            <button
              @click="removeQuickCommand(index)"
              class="opacity-0 group-hover:opacity-100 transition-opacity ml-1">
              <XIcon class="size-3 text-red-500 hover:text-red-700" />
            </button>
          </div>
          <div v-if="quickCommands.length === 0" class="text-sm text-muted-foreground">
            暂无常用命令，点击"添加"按钮创建
          </div>
        </div>
      </div>

      <!-- 历史记录面板 -->
      <div v-if="showHistory" class="mb-4 p-4 bg-muted rounded-lg max-h-[300px] overflow-auto">
        <div class="flex items-center justify-between mb-3">
          <h3 class="text-sm font-semibold">命令历史 ({{ commandHistory.length }})</h3>
          <div class="flex gap-2">
            <input
              v-model="historySearch"
              placeholder="搜索命令..."
              class="px-2 py-1 text-sm border rounded w-48" />
            <button class="btn btn-xs btn-outline" @click="clearHistory">
              <Trash2Icon class="size-3" />
              清空
            </button>
          </div>
        </div>
        <div class="space-y-1">
          <div
            v-for="(cmd, index) in filteredHistory"
            :key="index"
            @click="currentCommand = cmd"
            class="flex items-center gap-2 px-3 py-2 bg-background hover:bg-primary/10 rounded cursor-pointer text-sm group">
            <ClockIcon class="size-3 text-muted-foreground" />
            <code class="flex-1 font-mono">{{ cmd }}</code>
            <button
              @click.stop="executeQuickCommand(cmd)"
              class="opacity-0 group-hover:opacity-100 transition-opacity">
              <PlayIcon class="size-3 text-primary" />
            </button>
          </div>
          <div
            v-if="filteredHistory.length === 0"
            class="text-sm text-muted-foreground text-center py-4">
            {{ historySearch ? '未找到匹配的命令' : '暂无历史记录' }}
          </div>
        </div>
      </div>

      <!-- 终端输出区域 -->
      <div
        ref="terminalOutput"
        class="bg-black/90 rounded-lg flex-1 p-4 font-mono text-sm min-h-96 text-green-400 overflow-auto terminal-output"
        @click="focusInput">
        <!-- 历史命令及输出 -->
        <div v-for="(log, index) in terminalLogs" :key="index" class="mb-3">
          <!-- 命令行 -->
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-blue-400">{{ log.timestamp }}</span>
            <span class="text-yellow-400"
              >{{ currentServer.username }}@{{ currentServer.host }}</span
            >
            <span class="text-gray-400">$</span>
            <span class="text-green-400">{{ log.command }}</span>
          </div>

          <!-- 输出内容 -->
          <div v-if="log.outputs.length > 0" class="ml-4 mt-1">
            <div
              v-for="(output, outputIndex) in log.outputs"
              :key="outputIndex"
              class="text-gray-300 whitespace-pre-wrap">
              <span v-html="renderOutput(output)"></span>
            </div>
          </div>
        </div>

        <!-- 当前输入行 -->
        <div class="flex items-center gap-2" v-if="!isExecuting">
          <span class="text-blue-400">{{ currentTime }}</span>
          <span class="text-yellow-400">{{ currentServer.username }}@{{ currentServer.host }}</span>
          <span class="text-gray-400">$</span>
          <input
            ref="commandInput"
            v-model="currentCommand"
            @keydown="handleKeyDown"
            @keyup.enter="handleExecuteCommand"
            :disabled="isExecuting"
            class="flex-1 bg-transparent outline-none text-green-400 disabled:opacity-50 disabled:cursor-not-allowed"
            placeholder="输入命令 (↑↓ 历史记录, Tab 自动补全, Ctrl+C 中断)" />
        </div>

        <!-- 执行中提示 -->
        <div v-else class="flex items-center gap-2 text-yellow-400 animate-pulse">
          <LoaderIcon class="size-4 animate-spin" />
          <span>命令执行中...</span>
        </div>
      </div>

      <!-- 快捷键提示 -->
      <div class="mt-2 flex gap-4 text-xs text-muted-foreground flex-wrap">
        <span><kbd class="px-1.5 py-0.5 bg-muted rounded">↑↓</kbd> 历史记录</span>
        <span><kbd class="px-1.5 py-0.5 bg-muted rounded">Tab</kbd> 自动补全</span>
        <span><kbd class="px-1.5 py-0.5 bg-muted rounded">Ctrl+L</kbd> 清屏</span>
        <span><kbd class="px-1.5 py-0.5 bg-muted rounded">Ctrl+C</kbd> 中断</span>
        <span><kbd class="px-1.5 py-0.5 bg-muted rounded">Ctrl+K</kbd> 清空输入</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  Server as ServerIcon,
  AlertCircle as AlertCircleIcon,
  Terminal as TerminalIcon,
  History as HistoryIcon,
  Trash2 as Trash2Icon,
  Plus as PlusIcon,
  X as XIcon,
  Command as CommandIcon,
  Clock as ClockIcon,
  Play as PlayIcon,
  Loader as LoaderIcon,
} from 'lucide-vue-next'
import { computed, ref, watch, nextTick, onMounted, onUnmounted } from 'vue'

import { renderOutput } from '@/utils'

const serverStore = useServerStore()
const terminalStore = useTerminalStore()

const { currentServer } = storeToRefs(serverStore)
const { terminalLogs, currentCommand } = storeToRefs(terminalStore)

const terminalOutput = ref<HTMLDivElement>()
const commandInput = ref<HTMLInputElement>()

// UI 状态
const showQuickCommands = ref(false)
const showHistory = ref(false)
const showAddCommand = ref(false)

// 历史记录相关
const commandHistory = ref<string[]>([])
const historyIndex = ref(-1)
const historySearch = ref('')
const tempCommand = ref('') // 临时保存当前输入

// 常用命令
const quickCommands = ref<Array<{ name: string; command: string }>>([
  { name: '系统信息', command: 'uname -a' },
  { name: '磁盘使用', command: 'df -h' },
  { name: '内存使用', command: 'free -h' },
  { name: '进程列表', command: 'ps aux | head -20' },
  { name: '网络连接', command: 'netstat -tulpn' },
])

const newCommandName = ref('')
const newCommandValue = ref('')

// 当前时间
const currentTime = ref(new Date().toLocaleTimeString())
const updateTime = () => {
  currentTime.value = new Date().toLocaleTimeString()
}

let timeInterval: number | null = null

onMounted(() => {
  timeInterval = window.setInterval(updateTime, 1000)
  loadCommandHistory()
  loadQuickCommands()
})

onUnmounted(() => {
  if (timeInterval) {
    clearInterval(timeInterval)
  }
})

// 是否有命令正在执行
const isExecuting = computed(() => {
  return terminalLogs.value.some((log) => log.isExecuting)
})

// 过滤后的历史记录
const filteredHistory = computed(() => {
  if (!historySearch.value) {
    return commandHistory.value.slice().reverse().slice(0, 50)
  }
  return commandHistory.value
    .filter((cmd) => cmd.toLowerCase().includes(historySearch.value.toLowerCase()))
    .reverse()
    .slice(0, 50)
})

// 执行命令
const handleExecuteCommand = () => {
  if (currentCommand.value.trim() && !isExecuting.value) {
    // 添加到历史记录
    addToHistory(currentCommand.value.trim())
    terminalStore.executeCommand()
    historyIndex.value = -1
    tempCommand.value = ''
  }
}

// 执行快捷命令
const executeQuickCommand = (command: string) => {
  currentCommand.value = command
  nextTick(() => {
    handleExecuteCommand()
  })
}

// 添加到历史记录
const addToHistory = (command: string) => {
  // 避免重复的连续命令
  if (commandHistory.value[commandHistory.value.length - 1] !== command) {
    commandHistory.value.push(command)
    // 限制历史记录数量
    if (commandHistory.value.length > 1000) {
      commandHistory.value = commandHistory.value.slice(-1000)
    }
    saveCommandHistory()
  }
}

// 键盘事件处理
const handleKeyDown = (e: KeyboardEvent) => {
  // 上箭头 - 上一条历史
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    if (commandHistory.value.length === 0) return

    if (historyIndex.value === -1) {
      tempCommand.value = currentCommand.value
      historyIndex.value = commandHistory.value.length - 1
    } else if (historyIndex.value > 0) {
      historyIndex.value--
    }
    currentCommand.value = commandHistory.value[historyIndex.value]
  }

  // 下箭头 - 下一条历史
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    if (historyIndex.value === -1) return

    if (historyIndex.value < commandHistory.value.length - 1) {
      historyIndex.value++
      currentCommand.value = commandHistory.value[historyIndex.value]
    } else {
      historyIndex.value = -1
      currentCommand.value = tempCommand.value
    }
  }

  // Ctrl+L - 清屏
  if (e.ctrlKey && e.key === 'l') {
    e.preventDefault()
    terminalStore.clearTerminal()
  }

  // Ctrl+C - 中断（清空当前输入）
  if (e.ctrlKey && e.key === 'c') {
    e.preventDefault()
    currentCommand.value = ''
    historyIndex.value = -1
    tempCommand.value = ''
  }

  // Ctrl+K - 清空输入
  if (e.ctrlKey && e.key === 'k') {
    e.preventDefault()
    currentCommand.value = ''
  }

  // Tab - 简单的自动补全（可以根据需求扩展）
  if (e.key === 'Tab') {
    e.preventDefault()
    const cmd = currentCommand.value.trim()
    const commonCommands = [
      'ls',
      'cd',
      'pwd',
      'cat',
      'grep',
      'find',
      'docker',
      'systemctl',
      'vim',
      'nano',
    ]
    const matches = commonCommands.filter((c) => c.startsWith(cmd))
    if (matches.length === 1) {
      currentCommand.value = matches[0] + ' '
    }
  }
}

// 添加常用命令
const handleAddQuickCommand = () => {
  if (newCommandName.value.trim() && newCommandValue.value.trim()) {
    quickCommands.value.push({
      name: newCommandName.value.trim(),
      command: newCommandValue.value.trim(),
    })
    saveQuickCommands()
    newCommandName.value = ''
    newCommandValue.value = ''
    showAddCommand.value = false
  }
}

// 删除常用命令
const removeQuickCommand = (index: number) => {
  quickCommands.value.splice(index, 1)
  saveQuickCommands()
}

// 清空历史记录
const clearHistory = () => {
  if (confirm('确定要清空所有历史记录吗？')) {
    commandHistory.value = []
    historyIndex.value = -1
    saveCommandHistory()
  }
}

// 聚焦输入框
const focusInput = () => {
  commandInput.value?.focus()
}

// 自动滚动到底部
const scrollToBottom = () => {
  nextTick(() => {
    if (terminalOutput.value) {
      terminalOutput.value.scrollTop = terminalOutput.value.scrollHeight
    }
  })
}

// 本地存储
const HISTORY_KEY = 'ssh_terminal_history'
const QUICK_COMMANDS_KEY = 'ssh_quick_commands'
const autoScrollDown = ref(true)

const saveCommandHistory = () => {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(commandHistory.value))
  } catch (e) {
    console.error('Failed to save command history:', e)
  }
}

const loadCommandHistory = () => {
  try {
    const saved = localStorage.getItem(HISTORY_KEY)
    if (saved) {
      commandHistory.value = JSON.parse(saved)
    }
  } catch (e) {
    console.error('Failed to load command history:', e)
  }
}

const saveQuickCommands = () => {
  try {
    localStorage.setItem(QUICK_COMMANDS_KEY, JSON.stringify(quickCommands.value))
  } catch (e) {
    console.error('Failed to save quick commands:', e)
  }
}

const loadQuickCommands = () => {
  try {
    const saved = localStorage.getItem(QUICK_COMMANDS_KEY)
    if (saved) {
      quickCommands.value = JSON.parse(saved)
    }
  } catch (e) {
    console.error('Failed to load quick commands:', e)
  }
}

// 监听日志变化，自动滚动
watch(
  () => terminalLogs.value,
  () => {
    if (autoScrollDown.value) {
      scrollToBottom()
    }
  },
  { deep: true },
)

// 聚焦输入框
watch(
  () => currentServer.value,
  () => {
    nextTick(() => {
      focusInput()
    })
  },
)

onMounted(() => {
  document.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.key === 'c') {
      e.preventDefault()
      terminalStore.stopCommand()
    }
  })
})
</script>

<style scoped>
.terminal-output {
  scrollbar-width: thin;
  scrollbar-color: rgba(75, 85, 99, 0.5) transparent;
}

.terminal-output::-webkit-scrollbar {
  width: 8px;
}

.terminal-output::-webkit-scrollbar-track {
  background: transparent;
}

.terminal-output::-webkit-scrollbar-thumb {
  background-color: rgba(75, 85, 99, 0.5);
  border-radius: 4px;
}

.terminal-output::-webkit-scrollbar-thumb:hover {
  background-color: rgba(75, 85, 99, 0.7);
}

kbd {
  font-family: monospace;
  font-size: 0.85em;
}
</style>
