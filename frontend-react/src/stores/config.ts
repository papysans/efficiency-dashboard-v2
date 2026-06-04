// 可配置项（persist localStorage）：高管大屏 ROI 用人天单价折算 ¥。
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface ConfigState {
  /** 人天单价（¥/人天），用于把节省人天折算成节省成本。默认 ¥2000（保守估算，老板可调） */
  costPerPersonDay: number
  setCostPerPersonDay: (v: number) => void
}

export const useConfigStore = create<ConfigState>()(
  persist(
    (set) => ({
      costPerPersonDay: 2000,
      setCostPerPersonDay: (v) => set({ costPerPersonDay: v }),
    }),
    { name: 'kanban-config' },
  ),
)
