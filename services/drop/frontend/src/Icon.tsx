import type { ReactNode } from 'react';

type Name = 'plus'|'settings'|'send'|'stop'|'back'|'refresh'|'more'|'copy'|'download'|'close'|'trash'|'clock'|'check'|'file';
export function Icon({ name }: { name: Name }) {
  const paths: Record<Name, ReactNode> = {
    plus: <path d="M12 5v14M5 12h14"/>, settings: <><path d="M4 7h5m4 0h7M4 17h8m4 0h4"/><circle cx="11" cy="7" r="2"/><circle cx="14" cy="17" r="2"/></>,
    send: <path d="M12 19V5m-6 6 6-6 6 6"/>, stop: <rect x="7" y="7" width="10" height="10" rx="2" fill="currentColor" stroke="none"/>, back: <path d="m14 6-6 6 6 6"/>,
    refresh: <><path d="M20 11a8 8 0 1 0-2.34 5.66"/><path d="M20 4v7h-7"/></>, more: <><circle cx="5" cy="12" r=".8" fill="currentColor" stroke="none"/><circle cx="12" cy="12" r=".8" fill="currentColor" stroke="none"/><circle cx="19" cy="12" r=".8" fill="currentColor" stroke="none"/></>,
    copy: <><rect x="8" y="8" width="11" height="11" rx="2"/><path d="M16 5V4a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h1"/></>, download: <><path d="M12 3v12m0 0 4-4m-4 4-4-4"/><path d="M4 18v2h16v-2"/></>,
    close: <path d="m6 6 12 12M18 6 6 18"/>, trash: <><path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7"/><path d="M10 11v6m4-6v6"/></>, clock: <><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></>, check: <path d="m5 12 4 4L19 6"/>, file: <><path d="M7 3h7l4 4v14H7z"/><path d="M14 3v5h5"/></>,
  };
  return <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>;
}
