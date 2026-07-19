import type { LucideIcon } from 'lucide-react';
import { Boxes, Gauge, HardDriveUpload, Sparkles } from 'lucide-react';

export type HomeHubModule = {
  id: string;
  title: string;
  description: string;
  path: string;
  icon: LucideIcon;
  accent: string;
  requiredPermissions: string[];
  status: 'available' | 'building';
};

export const modules: HomeHubModule[] = [
  {
    id: 'overview',
    title: '概览',
    description: '你的服务、节点和最近活动。',
    path: '/',
    icon: Gauge,
    accent: '#91f2c2',
    requiredPermissions: ['control.dashboard.read'],
    status: 'available',
  },
  {
    id: 'drop',
    title: 'Drop',
    description: '在设备、朋友和 Agent 之间传递原始文件。',
    path: '/drop',
    icon: HardDriveUpload,
    accent: '#8cc8ff',
    requiredPermissions: ['drop.item.read'],
    status: 'available',
  },
  {
    id: 'ai',
    title: 'AI Gateway',
    description: '模型路由、用量和流式调用状态。',
    path: '/ai',
    icon: Sparkles,
    accent: '#ff9eb5',
    requiredPermissions: ['ai.model.fast'],
    status: 'available',
  },
];

export const systemModule = {
  id: 'services',
  title: '服务',
  icon: Boxes,
};
