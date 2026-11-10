import { parseDelimited, parseInvitationSheet } from './invitationImport';
import type { Season } from '@/types';

const seasons = [
  { id: 's1', name: 'Summer 2025/26', slug: 'summer-2025-26' },
  { id: 's2', name: 'Winter 2026', slug: 'winter-2026' },
] as Season[];

it('reads quoted cells, line breaks inside quotes and tab-separated sheets', () => {
  expect(parseDelimited('a,"b, c","say ""hi"""\r\n1,2,3\n\n')).toEqual([
    ['a', 'b, c', 'say "hi"'],
    ['1', '2', '3'],
  ]);
  expect(parseDelimited('a\tb\n"x\ny"\tz')).toEqual([
    ['a', 'b'],
    ['x\ny', 'z'],
  ]);
});

it('matches columns in any order and seasons by name or slug', () => {
  const rows = parseInvitationSheet(
    [
      'Email,Season,Name,Role,Mentor',
      'ALEX@Example.com,summer 2025/26,Alex Chen,Student,priya@example.com',
      'sam@example.com,winter-2026,Sam Lee,mentor,',
    ].join('\n'),
    seasons,
  );
  expect(rows).toMatchObject([
    {
      line: 2,
      email: 'alex@example.com',
      role: 'student',
      mentor: 'priya@example.com',
      season: { id: 's1' },
      problem: '',
    },
    { line: 3, role: 'mentor', season: { id: 's2' }, problem: '' },
  ]);
});

it('flags each kind of bad row and leaves the good ones ready', () => {
  const rows = parseInvitationSheet(
    [
      'name,role,email,mentor,season,slug',
      ',student,a@example.com,,Summer 2025/26',
      'B,student,not-an-email,,Summer 2025/26',
      'C,teacher,c@example.com,,Summer 2025/26',
      'D,mentor,d@example.com,Priya,Summer 2025/26',
      'E,student,e@example.com,,',
      'F,student,f@example.com,,Autumn 2030',
      'G,student,g@example.com,,Summer 2025/26',
      'G again,student,G@example.com,,summer-2025-26',
      'G elsewhere,student,g@example.com,,Winter 2026',
      'H,student,h@example.com,,Winter 2026,Not A Slug',
      'I,student,i@example.com,,Winter 2026,@ok-name',
    ].join('\n'),
    seasons,
  ) as ReturnType<typeof parseInvitationSheet> & object[];
  expect((rows as { problem: string }[]).map((row) => row.problem)).toEqual([
    'Add a name.',
    'Add a valid email address.',
    'Role must be student, mentor or coordinator.',
    'Only students can be given a mentor.',
    'Add a season.',
    'No open season called "Autumn 2030".',
    '',
    'This email is already listed for this season.',
    '',
    'Slugs use lowercase letters, numbers and single hyphens.',
    '',
  ]);
  expect((rows as { slug: string }[]).at(-1)?.slug).toBe('ok-name');
});

it('explains a sheet it cannot read', () => {
  expect(parseInvitationSheet('', seasons)).toBe('This file is empty.');
  expect(parseInvitationSheet('name,email\nA,a@example.com', seasons)).toBe(
    'The first row must name the columns; missing: role, season.',
  );
  expect(parseInvitationSheet('name,role,email,season', seasons)).toBe(
    'This file has no rows under its header.',
  );
});
