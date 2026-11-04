import type { Participation } from '@/api/generated/models';
import { activitySeasons } from '@/activityPeriod';
import type { Season } from '@/types';

const season = (id: string) => ({ id, name: id }) as Season;
const seasons = ['student', 'promoted', 'mentor', 'test'].map(season);
const participation = (
  seasonId: string,
  role: Participation['role'],
  periodRoles = [role],
) =>
  ({
    seasonId,
    role,
    periods: periodRoles.map((periodRole) => ({ role: periodRole })),
  }) as Participation;

const history = [
  participation('student', 'student'),
  // A student promoted to mentor during the season keeps that season.
  participation('promoted', 'mentor', ['student', 'mentor']),
  participation('mentor', 'mentor'),
];

it('lists only seasons where the member was a student', () => {
  expect(activitySeasons(seasons, history, {}).map((item) => item.id)).toEqual([
    'student',
    'promoted',
  ]);
});

it('lists every season the member took part in with anyRole', () => {
  expect(
    activitySeasons(seasons, history, { anyRole: true }).map((item) => item.id),
  ).toEqual(['student', 'promoted', 'mentor']);
});

it('offers nothing beyond the selected season without participation', () => {
  expect(activitySeasons(seasons, [], {})).toEqual([]);
  expect(
    activitySeasons(seasons, [], { selectedId: 'test' }).map((item) => item.id),
  ).toEqual(['test']);
});
