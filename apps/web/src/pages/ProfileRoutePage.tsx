import { Navigate, useLocation } from 'react-router-dom';

import { ProfilePage } from '@/pages/ProfilePage';

export function ProfileRoutePage() {
  const location = useLocation();
  const legacyUser = new URLSearchParams(location.search).get('user');
  if (legacyUser)
    return (
      <Navigate to={`/people/${encodeURIComponent(legacyUser)}`} replace />
    );
  return <ProfilePage />;
}
