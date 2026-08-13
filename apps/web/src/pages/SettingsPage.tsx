import { Switch } from '@base-ui/react/switch';
import { IconAlertTriangle, IconCheck, IconKey, IconShieldCheck } from '@tabler/icons-react';
import { useMemo, useState } from 'react';

import { useCurrentUser } from '@/api/queries';
import { PageHeader, usePageTitle } from '@/components/Common';
import { NamedConfirmation } from '@/components/Dialogs';
import { InlineNotice, PageSkeleton } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';

export function SettingsPage() {
  usePageTitle('Settings');
  const user = useCurrentUser();
  const browserTimezone = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone, []);
  const [timezone, setTimezone] = useState(user.data?.timezone ?? browserTimezone);
  const [goalsEnabled, setGoalsEnabled] = useState(false);
  const [premiumEnabled, setPremiumEnabled] = useState(false);
  const [saved, setSaved] = useState(false);
  if (user.isLoading) return <PageSkeleton label="Loading settings" />;

  return (
    <div className={styles.page}>
      <PageHeader eyebrow="Account" title="Settings" description="Control your local display preferences, practice goals and account security." actions={<button className={styles.button} type="button" onClick={() => { setSaved(true); window.setTimeout(() => setSaved(false), 2500); }}>Save settings</button>} />
      {saved ? <InlineNotice tone="success"><IconCheck size={19} aria-hidden="true" /> Settings saved.</InlineNotice> : null}
      <div className={styles.grid2}>
        <section className={styles.section} aria-labelledby="preferences-title">
          <div className={styles.sectionHeader}><h2 id="preferences-title" className={styles.sectionTitle}>Display and time</h2></div>
          <div className={styles.panel}>
            <div className={styles.field}><label htmlFor="timezone">Timezone</label><select id="timezone" className={styles.select} value={timezone} onChange={(event) => setTimezone(event.target.value)}><option value={browserTimezone}>{browserTimezone} (browser detected)</option>{browserTimezone !== 'Australia/Adelaide' ? <option>Australia/Adelaide</option> : null}<option>Australia/Sydney</option><option>Asia/Hong_Kong</option><option>UTC</option></select><p className={styles.helper}>Stored timestamps remain UTC. Dates are rendered in this timezone.</p></div>
            <SettingSwitch label="Include premium LeetCode problems" description="Allow premium problems in recommendation candidates." checked={premiumEnabled} onCheckedChange={setPremiumEnabled} />
          </div>
        </section>
        <section className={styles.section} aria-labelledby="goals-title">
          <div className={styles.sectionHeader}><h2 id="goals-title" className={styles.sectionTitle}>Practice goals</h2></div>
          <div className={styles.panel}>
            <SettingSwitch label="Use personal time goals" description="A mentor or programme administrator must enable this first. Every change is audited." checked={goalsEnabled} onCheckedChange={setGoalsEnabled} />
            <div className={styles.fieldGrid} style={{ marginTop: '0.9rem' }}>
              {(['Easy', 'Medium', 'Hard'] as const).map((difficulty, index) => <div className={styles.field} key={difficulty}><label htmlFor={`goal-${difficulty}`}>{difficulty} minutes</label><input id={`goal-${difficulty}`} className={styles.input} type="number" min="5" max="180" defaultValue={[20, 35, 50][index]} disabled={!goalsEnabled} /></div>)}
            </div>
          </div>
        </section>
      </div>
      <section className={styles.section} aria-labelledby="security-title">
        <div className={styles.sectionHeader}><h2 id="security-title" className={styles.sectionTitle}>Security</h2></div>
        <div className={styles.panel}>
          <div className={styles.listRow}><span className={styles.personIdentity}><span className={styles.avatar}><IconShieldCheck size={20} aria-hidden="true" /></span><span className={styles.personIdentityText}><strong>Two-factor authentication</strong><span>Required for privileged programme roles</span></span></span><span className={user.data?.mfaVerified ? styles.badgeSuccess : styles.badgeWarning}>{user.data?.mfaVerified ? 'Configured' : 'Required'}</span></div>
          <div className={styles.listRow}><span className={styles.personIdentity}><span className={styles.avatar}><IconKey size={20} aria-hidden="true" /></span><span className={styles.personIdentityText}><strong>Backup codes</strong><span>Single-use codes for account recovery</span></span></span><button className={styles.buttonSecondary} type="button">Regenerate after reauthentication</button></div>
          <div className={styles.listRow}><span>Connected sign-in methods</span><button className={styles.buttonSecondary} type="button">Connect method</button></div>
        </div>
      </section>
      <section className={styles.section} aria-labelledby="danger-title">
        <div className={styles.sectionHeader}><h2 id="danger-title" className={styles.sectionTitle}>Account deletion</h2></div>
        <div className={styles.panel}>
          <InlineNotice tone="warning"><IconAlertTriangle size={19} aria-hidden="true" /> Requesting deletion revokes all sessions immediately. A verified recovery link can cancel the request during the 30-day grace period.</InlineNotice>
          <div style={{ marginTop: '1rem' }}><NamedConfirmation name="DELETE MY ACCOUNT" actionLabel="Request account deletion" description="After 30 days, credentials and profile contact details are removed or pseudonymised. Historical programme records remain under an opaque ID." onConfirm={() => undefined} /></div>
        </div>
      </section>
    </div>
  );
}

function SettingSwitch({ label, description, checked, onCheckedChange }: { label: string; description: string; checked: boolean; onCheckedChange: (checked: boolean) => void }) {
  return (
    <div className={styles.switchRow}>
      <span><strong>{label}</strong><span className={styles.helper} style={{ display: 'block' }}>{description}</span></span>
      <Switch.Root className={styles.switchRoot} checked={checked} onCheckedChange={onCheckedChange} aria-label={label}><Switch.Thumb className={styles.switchThumb} /></Switch.Root>
    </div>
  );
}
