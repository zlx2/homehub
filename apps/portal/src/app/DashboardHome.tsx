import {
  Bot,
  Box,
  Boxes,
  ChevronRight,
  Clock3,
  Cpu,
  Droplets,
  Gauge,
  HardDrive,
  House,
  KeyRound,
  MemoryStick,
  Network,
  Search,
  Send,
  ServerCog,
  ShieldCheck,
  X,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import './dashboard-home.css';

type Status = 'healthy' | 'warning' | 'stopped';
type Filter = 'all' | 'warning' | 'stopped';

type OverviewService = {
  id: string;
  name?: string;
  route?: string;
  status?: { state?: string };
};

type OverviewResponse = {
  summary?: { total_services?: number; healthy_services?: number };
  services?: OverviewService[];
};

type Service = {
  id: string;
  catalogId?: string;
  name: string;
  summary: string;
  status: Status;
  statusLabel: string;
  section: 'common' | 'other';
  icon: LucideIcon;
  tone: 'sky' | 'cyan' | 'teal' | 'blue' | 'slate';
  route?: string;
  spa?: boolean;
};

const serviceDefinitions: Service[] = [
  { id: 'hermes', catalogId: 'hermes-terminal', name: 'Hermes', summary: '网页终端', status: 'stopped', statusLabel: '检查中', section: 'common', icon: Bot, tone: 'cyan', route: '/hermes/' },
  { id: 'drop', catalogId: 'drop', name: 'Drop', summary: '文本与原始文件', status: 'stopped', statusLabel: '检查中', section: 'common', icon: Droplets, tone: 'sky', route: '/drop', spa: true },
  { id: 'telegram', name: 'Telegram', summary: '消息转发到 Drop', status: 'healthy', statusLabel: '运行中', section: 'common', icon: Send, tone: 'blue' },
  { id: 'ai', catalogId: 'ai-gateway', name: 'AI Gateway', summary: 'DeepSeek · OpenCode', status: 'stopped', statusLabel: '检查中', section: 'common', icon: Network, tone: 'teal' },
  { id: 'server', catalogId: 'server-monitor', name: '服务器面板', summary: '资源与容器监控', status: 'stopped', statusLabel: '检查中', section: 'other', icon: ServerCog, tone: 'cyan', route: '/server/' },
  { id: 'control', catalogId: 'homehub-control', name: 'HomeHub Control', summary: '身份、权限与服务目录', status: 'stopped', statusLabel: '检查中', section: 'other', icon: ShieldCheck, tone: 'sky', route: '/security', spa: true },
  { id: 'edge', catalogId: 'homehub-edge', name: 'HomeHub Edge', summary: 'Traefik 入口与路由', status: 'stopped', statusLabel: '检查中', section: 'other', icon: Gauge, tone: 'slate' },
];

const filters: Array<{ value: Filter; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'warning', label: '异常' },
  { value: 'stopped', label: '未启动' },
];

function normalizeStatus(state?: string): Pick<Service, 'status' | 'statusLabel'> {
  if (state === 'healthy') return { status: 'healthy', statusLabel: '正常' };
  if (state === 'degraded' || state === 'warning') return { status: 'warning', statusLabel: '异常' };
  return { status: 'stopped', statusLabel: state ? '不可用' : '检查中' };
}

function Metric({ icon: Icon, label, value, progress, warning = false }: {
  icon: LucideIcon;
  label: string;
  value: string;
  progress: number;
  warning?: boolean;
}) {
  return <div className={`dashboard-metric ${warning ? 'dashboard-metric-warning' : ''}`}>
    <div className="dashboard-metric-topline"><span><Icon size={18} strokeWidth={1.8}/>{label}</span><strong>{value}</strong></div>
    <div className="dashboard-metric-track" aria-hidden="true"><span style={{ width: `${progress}%` }}/></div>
  </div>;
}

function ServiceCard({ service, open }: { service: Service; open: (service: Service) => void }) {
  const Icon = service.icon;
  return <button className="dashboard-service-card" type="button" data-status={service.status} disabled={!service.route} onClick={() => open(service)} aria-label={service.route ? `打开 ${service.name}` : `${service.name} 仅展示状态`}>
    <span className={`dashboard-service-icon dashboard-tone-${service.tone}`}><Icon size={24} strokeWidth={1.8}/></span>
    <span className="dashboard-service-copy">
      <span className="dashboard-service-title"><strong>{service.name}</strong><span className={`dashboard-service-state dashboard-state-${service.status}`}><i/>{service.statusLabel}</span></span>
      <span className="dashboard-service-summary">{service.summary}</span>
    </span>
    {service.route && <ChevronRight className="dashboard-service-chevron" size={20} strokeWidth={1.7}/>} 
  </button>;
}

export function DashboardHome({ name, navigate }: { name: string; navigate: (path: string) => void }) {
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');
  const [overview, setOverview] = useState<OverviewResponse>();

  useEffect(() => {
    fetch('/api/control/v1/overview')
      .then((response) => response.ok ? response.json() : Promise.reject())
      .then(setOverview)
      .catch(() => setOverview({ summary: { total_services: 0, healthy_services: 0 }, services: [] }));
  }, []);

  const services = useMemo(() => {
    const states = new Map((overview?.services ?? []).map((service) => [service.id, service.status?.state]));
    return serviceDefinitions.map((service) => service.catalogId ? { ...service, ...normalizeStatus(states.get(service.catalogId)) } : service);
  }, [overview]);

  const visibleServices = useMemo(() => {
    const normalized = query.trim().toLocaleLowerCase('zh-CN');
    return services.filter((service) => (filter === 'all' || service.status === filter) && (!normalized || `${service.name} ${service.summary}`.toLocaleLowerCase('zh-CN').includes(normalized)));
  }, [filter, query, services]);

  const common = visibleServices.filter((service) => service.section === 'common');
  const other = visibleServices.filter((service) => service.section === 'other');
  const unhealthy = services.filter((service) => service.catalogId && service.status !== 'healthy').length;

  function open(service: Service) {
    if (!service.route) return;
    if (service.spa) navigate(service.route);
    else location.assign(service.route);
  }

  return <div className="dashboard-page">
    <div className="dashboard-shell">
      <header className="dashboard-topbar">
        <button className="dashboard-brand" type="button" onClick={() => navigate('/')}><span><House size={24} strokeWidth={1.8}/></span>HomeHub</button>
        <div className="dashboard-topbar-actions">
          <div className={`dashboard-health-summary ${unhealthy ? 'warning' : ''}`}><i/>{unhealthy ? `${unhealthy} 项需关注` : '核心服务正常'}</div>
          <button className="dashboard-avatar" type="button" onClick={() => navigate('/security')} aria-label="账号与安全">{name.trim().charAt(0).toUpperCase() || 'L'}</button>
        </div>
      </header>

      <main>
        <section className="dashboard-server-card" aria-label="服务器概况">
          <div className="dashboard-server-identity"><span className="dashboard-server-icon"><Box size={30} strokeWidth={1.45}/></span><span className="dashboard-server-copy"><span className="dashboard-server-name"><strong>hermes</strong><i/></span><span>4 核 · 3.6 GB</span></span></div>
          <Metric icon={Cpu} label="CPU" value="18%" progress={18}/>
          <Metric icon={MemoryStick} label="内存" value="31%" progress={31}/>
          <Metric icon={HardDrive} label="磁盘" value="75%" progress={75} warning/>
          <div className="dashboard-uptime"><Clock3 size={20} strokeWidth={1.75}/><span><small>已运行</small><strong>12 天</strong></span></div>
        </section>

        <section className="dashboard-services-panel" aria-labelledby="dashboard-services-title">
          <div className="dashboard-services-heading">
            <div className="dashboard-heading-copy"><h1 id="dashboard-services-title">服务</h1><span>{visibleServices.length} 个服务</span></div>
            <div className="dashboard-service-controls">
              <label className="dashboard-search-field"><Search size={19} strokeWidth={1.8}/><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索服务…" aria-label="搜索服务"/>{query && <button type="button" onClick={() => setQuery('')} aria-label="清除搜索"><X size={16}/></button>}</label>
              <div className="dashboard-filter-group" aria-label="按状态筛选服务">{filters.map((item) => <button key={item.value} className={filter === item.value ? 'active' : ''} type="button" onClick={() => setFilter(item.value)} aria-pressed={filter === item.value}>{item.label}</button>)}</div>
            </div>
          </div>

          {common.length > 0 && <div className="dashboard-service-section"><h2><span/>常用</h2><div className="dashboard-service-grid">{common.map((service) => <ServiceCard key={service.id} service={service} open={open}/>)}</div></div>}
          {other.length > 0 && <div className="dashboard-service-section"><h2><span/>其他</h2><div className="dashboard-service-grid">{other.map((service) => <ServiceCard key={service.id} service={service} open={open}/>)}</div></div>}
          {visibleServices.length === 0 && <div className="dashboard-empty-state"><Boxes size={28} strokeWidth={1.5}/><strong>没有符合条件的服务</strong><span>换个关键词或筛选条件试试</span></div>}
        </section>
      </main>
    </div>
  </div>;
}
