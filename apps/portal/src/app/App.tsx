import { Bell, ChevronRight, Search, Settings2 } from 'lucide-react';
import type { CSSProperties } from 'react';
import { modules } from './modules';

export function App() {
  return (
    <div className="portal-shell">
      <aside className="sidebar" aria-label="主导航">
        <a className="brand" href="/" aria-label="HomeHub 首页">
          <span className="brand-mark">H</span>
          <span>HomeHub</span>
        </a>

        <nav className="desktop-navigation">
          {modules.map((module, index) => {
            const Icon = module.icon;
            return (
              <a className={index === 0 ? 'nav-item active' : 'nav-item'} href={module.path} key={module.id}>
                <Icon size={19} strokeWidth={1.8} />
                <span>{module.title}</span>
              </a>
            );
          })}
        </nav>

        <div className="sidebar-footer">
          <span className="connection-dot" />
          <div>
            <strong>HomeHub V2</strong>
            <span>正在重构</span>
          </div>
        </div>
      </aside>

      <main className="main-content">
        <header className="topbar">
          <div className="mobile-brand">
            <span className="brand-mark">H</span>
            <strong>HomeHub</strong>
          </div>
          <div className="topbar-actions">
            <button className="icon-button" aria-label="搜索"><Search size={19} /></button>
            <button className="icon-button" aria-label="通知"><Bell size={19} /></button>
            <button className="avatar" aria-label="账户">L</button>
          </div>
        </header>

        <div className="content-wrap">
          <section className="hero">
            <div>
              <span className="eyebrow">PERSONAL CONTROL PLANE</span>
              <h1>晚上好，Luna</h1>
              <p>一个入口，管理服务、设备与 Agent。</p>
            </div>
            <div className="hero-status">
              <span className="pulse" />
              V2 开发环境
            </div>
          </section>

          <section aria-labelledby="modules-title">
            <div className="section-heading">
              <div>
                <span className="eyebrow">MODULES</span>
                <h2 id="modules-title">你的空间</h2>
              </div>
              <button className="text-button"><Settings2 size={17} />管理模块</button>
            </div>

            <div className="module-grid">
              {modules.slice(1).map((module) => {
                const Icon = module.icon;
                return (
                  <a className="module-card" href={module.path} key={module.id}>
                    <div className="module-icon" style={{ '--module-accent': module.accent } as CSSProperties}>
                      <Icon size={23} strokeWidth={1.8} />
                    </div>
                    <div className="module-copy">
                      <div className="module-title-row">
                        <h3>{module.title}</h3>
                        <span className={`status-pill ${module.status}`}>{module.status === 'available' ? '可用' : '构建中'}</span>
                      </div>
                      <p>{module.description}</p>
                    </div>
                    <ChevronRight className="module-arrow" size={19} />
                  </a>
                );
              })}
            </div>
          </section>
        </div>
      </main>

      <nav className="mobile-navigation" aria-label="手机导航">
        {modules.slice(0, 4).map((module, index) => {
          const Icon = module.icon;
          return (
            <a className={index === 0 ? 'mobile-nav-item active' : 'mobile-nav-item'} href={module.path} key={module.id}>
              <Icon size={20} strokeWidth={1.8} />
              <span>{module.title}</span>
            </a>
          );
        })}
      </nav>
    </div>
  );
}
