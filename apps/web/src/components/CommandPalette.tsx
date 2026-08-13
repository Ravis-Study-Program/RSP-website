import { Dialog } from '@base-ui/react/dialog';
import { IconSearch, IconX } from '@tabler/icons-react';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';

import styles from '@/styles/App.module.css';

const commands = [
  { label: 'Dashboard', description: 'Your current workspace summary', to: '/dashboard' },
  { label: 'Practice', description: 'Recommendation and attempt history', to: '/practice' },
  { label: 'Mock interviews', description: 'Received, given and available interviews', to: '/mock-interviews' },
  { label: 'Seasons', description: 'Current and completed programmes', to: '/seasons' },
  { label: 'Graduates', description: 'Alumni directory', to: '/graduates' },
  { label: 'Profile', description: 'Public profile and private account details', to: '/profile' },
  { label: 'Settings', description: 'Timezone, practice goals and security', to: '/settings' },
  { label: 'Administration', description: 'Audited programme operations', to: '/admin' },
];

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const filtered = useMemo(() => commands.filter((command) => `${command.label} ${command.description}`.toLowerCase().includes(query.toLowerCase())), [query]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen((value) => !value);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  return (
    <Dialog.Root open={open} onOpenChange={(next) => { setOpen(next); if (!next) setQuery(''); }}>
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.dialogBackdrop} />
        <Dialog.Viewport className={styles.dialogViewport}>
          <Dialog.Popup className={styles.dialogPopup}>
            <div className={styles.dialogHeader}>
              <div><Dialog.Title className={styles.dialogTitle}>Quick navigation</Dialog.Title><Dialog.Description className={styles.dialogDescription}>Search pages available in your current role.</Dialog.Description></div>
              <Dialog.Close className={styles.iconButton} aria-label="Close quick navigation"><IconX size={18} aria-hidden="true" /></Dialog.Close>
            </div>
            <div className={styles.searchWrap} style={{ width: '100%' }}><IconSearch size={18} aria-hidden="true" /><label className={styles.visuallyHidden} htmlFor="command-search">Search pages</label><input id="command-search" className={styles.searchInput} autoComplete="off" placeholder="Search pages…" value={query} onChange={(event) => setQuery(event.target.value)} /></div>
            <nav aria-label="Quick navigation results" style={{ marginTop: '0.8rem' }}>
              <ul className={styles.cleanList}>
                {filtered.map((command) => <li key={command.to}><Link className={styles.resourceLink} to={command.to} onClick={() => setOpen(false)}><span><strong>{command.label}</strong><span className={styles.helper} style={{ display: 'block' }}>{command.description}</span></span><span aria-hidden="true">→</span></Link></li>)}
              </ul>
              {!filtered.length ? <p className={styles.muted}>No matching pages.</p> : null}
            </nav>
          </Dialog.Popup>
        </Dialog.Viewport>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
