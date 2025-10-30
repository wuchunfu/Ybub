<template>
  <dialog :open="open" class="dialog">
    <div class="dialog-body dialog-body-xl">
      <div class="dialog-header">
        <h3 class="dialog-title">命令执行结果</h3>
        <button class="dialog-close" @click="$emit('close')">
          <XIcon class="size-5" />
        </button>
      </div>

      <div class="dialog-content">
        <div
          class="bg-black/90 rounded-lg p-4 font-mono text-sm text-green-400 max-h-[500px] overflow-auto">
          <div class="text-blue-400 mb-2">$ {{ commandOutput.command }}</div>
          <!-- 输出内容 -->
          <div v-if="commandOutput.outputs.length > 0" class="ml-4 mt-1">
            <div
              v-for="(output, outputIndex) in commandOutput.outputs"
              :key="outputIndex"
              class="text-gray-300 whitespace-pre-wrap">
              <div v-html="renderOutput(output)"></div>
            </div>
          </div>
        </div>
      </div>

      <div class="dialog-footer">
        <button class="btn btn-primary" @click="$emit('close')">关闭</button>
      </div>
    </div>
  </dialog>
</template>

<script setup lang="ts">
import type { DockerOutput } from '@/types'
import { X as XIcon } from 'lucide-vue-next'

import { renderOutput } from '@/utils'

defineProps<{
  open: boolean
  commandOutput: DockerOutput
}>()

defineEmits(['close'])
</script>
