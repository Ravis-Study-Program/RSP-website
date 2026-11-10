import { duplicateNames } from '@/duplicateNames';

it('finds names shared by different people, counting each person once', () => {
  const dupes = duplicateNames([
    { id: '1', name: 'Maddie Student' },
    { id: '2', name: 'maddie student ' },
    { id: '3', name: 'Noah Williams' },
    { id: '3', name: 'Noah Williams' },
  ]);
  expect([...dupes]).toEqual(['maddie student']);
});
