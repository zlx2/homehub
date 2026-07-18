import type { LucideIcon } from 'lucide-react';
import { Bot, Boxes, Gauge, HardDriveUpload, Server, Sparkles } from 'lucide-react';

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
    id: 'hermes',
    title: 'Hermes',
    description: '随时打开你的服务器管家与历史会话。',
    path: '/hermes',
    icon: Bot,
    accent: '#f2d888',
    requiredPermissions: ['hermes.session.use'],
    status: 'building',
  },
  {
    id: 'server',
    title: '服务器',
    description: '资源、容器、网络和系统状态。',
    path: '/server',
    icon: Server,
    accent: '#c4a7ff',
    requiredPermissions: ['server.metrics.read'],
    status: 'building',
  },
  {
    id: 'ai',
    title: 'AI Gateway',
    description: '模型路由、用量和流式调用状态。',
    path: '/ai',
    icon: Sparkles,
    accent: '#ff9eb5',
    requiredPermissions: ['ai.gateway.read'],
    status: 'building',
  },
];

export const systemModule = {
  id: 'services',
  title: '服务',
  icon: Boxes,
};
