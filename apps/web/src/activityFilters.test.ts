import {
  activityFilterQuery,
  emptyActivityFilters,
  filterMocks,
  filterPractice,
} from './activityFilters';
import type { Attempt, MockInterview } from './types';

it('matches any selected topic or difficulty and combines groups', () => {
  const attempts = [
    {
      id: 'a',
      difficulty: 'Easy',
      categories: ['Arrays', 'Hash maps'],
      weekId: '1',
    },
    { id: 'b', difficulty: 'Hard', categories: ['Trees'], weekId: '2' },
    { id: 'c', difficulty: 'Medium', categories: ['Hash maps'], weekId: '1' },
  ] as Attempt[];
  expect(
    filterPractice(attempts, {
      ...emptyActivityFilters,
      difficulties: ['easy', 'hard'],
      categories: ['Hash maps', 'Trees'],
      weeks: ['1'],
    }).map((a) => a.id),
  ).toEqual(['a']);
  expect(
    activityFilterQuery({
      ...emptyActivityFilters,
      categories: ['Hash maps', 'Arrays'],
    }),
  ).toBe('&category=Hash+maps&category=Arrays');
});

it('combines result and interviewer selection', () => {
  const mocks = [
    { id: 'a', interviewer: { id: 'one' }, rounds: [{ score: 4 }] },
    { id: 'b', interviewer: { id: 'one' }, rounds: [{ score: 8 }] },
    { id: 'c', interviewer: { id: 'two' }, rounds: [{ score: 8 }] },
  ] as MockInterview[];
  expect(
    filterMocks(mocks, {
      ...emptyActivityFilters,
      interviewers: ['one'],
      passed: 'true',
    }).map((m) => m.id),
  ).toEqual(['b']);
});

it('matches any selected mock week and combines it with interviewer and result', () => {
  const mocks = [
    {
      id: 'a',
      weekId: '1',
      interviewer: { id: 'one' },
      rounds: [{ score: 8 }],
    },
    {
      id: 'b',
      weekId: '2',
      interviewer: { id: 'one' },
      rounds: [{ score: 8 }],
    },
    {
      id: 'c',
      weekId: '1',
      interviewer: { id: 'two' },
      rounds: [{ score: 8 }],
    },
    {
      id: 'd',
      weekId: '2',
      interviewer: { id: 'one' },
      rounds: [{ score: 4 }],
    },
    { id: 'e', interviewer: { id: 'one' }, rounds: [{ score: 8 }] },
  ] as MockInterview[];
  expect(
    filterMocks(mocks, {
      ...emptyActivityFilters,
      weeks: ['1', '2'],
      interviewers: ['one'],
      passed: 'true',
    }).map((mock) => mock.id),
  ).toEqual(['a', 'b']);
  expect(filterMocks(mocks, emptyActivityFilters)).toHaveLength(5);
});
