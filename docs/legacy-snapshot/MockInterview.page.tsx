import { useMemo, useState } from 'react';
import { Flex, Group, MultiSelect, SegmentedControl, Select, Stack, Text } from '@mantine/core';
import { useLocalStorage } from '@mantine/hooks';
import { useListMockInterview } from '@/generated/api/client';
import { useSeasonSlug } from '@/shared/hooks/useSeasonSlug';
import { useUserAndEnrollment } from '@/shared/hooks/useUserAndEnrollment';
import { createOptionsFilter } from '@/shared/table/globalFilters';
import { MockInterviewTable } from './MockInterviewTable/MockInterviewTable';
import classes from './MockInterview.module.css';

export enum MockInterviewsPreset {
  All = 'All',
  ReceivedMocks = 'Received Mocks',
  GivenMocks = 'Given Mocks',
}

export default function MockInterviewPage() {
  const { seasonSlug } = useSeasonSlug();
  const { userId, seasonId } = useUserAndEnrollment(seasonSlug);
  const [selectedIsPassResult, setSelectedIsPassResult] = useState<boolean | null>(null);
  const [selectedInterviewers, setSelectedInterviewers] = useState<string[]>([]);
  const [selectedMockInterviewsPreset, setSelectedMockInterviewsPreset] = useLocalStorage({
    key: 'mock-interviews-preset-v2',
    defaultValue: MockInterviewsPreset.ReceivedMocks,
  });

  const { data: mockInterviewsResponse, refetch: refetchMockInterviews } = useListMockInterview(
    {
      SeasonId: seasonId || undefined,
      IncludeCustom: true,
      IncludeLeetcode: true,
      IncludeBehavioural: true,
      UserIds: [userId],
    },
    { query: { enabled: userId !== '' } }
  );

  const resultOptions = [
    { label: 'Pass', value: 'true' },
    { label: 'Fail', value: 'false' },
  ];

  const interviewersOptions = [
    ...(mockInterviewsResponse?.responseBody?.mockInterviews || []).map((mockInterview) => {
      return {
        label: mockInterview.interviewer?.name ?? '',
        value: mockInterview.interviewer?.userId ?? '',
      };
    }),
    ...(mockInterviewsResponse?.responseBody?.mockInterviews || []).map((mockInterview) => {
      return {
        label: mockInterview.interviewee?.name ? `${mockInterview.interviewee.name}` : '',
        value: mockInterview.interviewee?.userId ?? '',
      };
    }),
  ];
  const distinctInterviewersOptions = Array.from(
    new Map(interviewersOptions.map((option) => [option.value, option])).values()
  );

  const filteredMockInterviews = useMemo(() => {
    const mocks = mockInterviewsResponse?.responseBody?.mockInterviews || [];
    return mocks.filter(
      (mock) =>
        (selectedIsPassResult === null &&
          selectedMockInterviewsPreset === MockInterviewsPreset.All &&
          selectedInterviewers.length === 0) ||
        ((selectedIsPassResult === null ||
          (mock.isPass !== null &&
            (typeof mock.isPass === 'boolean' ? mock.isPass : mock.isPass === 'true') ===
              selectedIsPassResult)) &&
          (selectedMockInterviewsPreset === MockInterviewsPreset.All ||
            (selectedMockInterviewsPreset === MockInterviewsPreset.ReceivedMocks &&
              mock.interviewee?.userId !== null &&
              userId === mock.interviewee?.userId) ||
            (selectedMockInterviewsPreset === MockInterviewsPreset.GivenMocks &&
              mock.interviewer?.userId !== null &&
              userId === mock.interviewer?.userId)) &&
          (selectedInterviewers.length === 0 ||
            (mock.interviewer?.userId != null &&
              selectedInterviewers.includes(mock.interviewer?.userId.toString()))))
    );
  }, [
    userId,
    seasonId,
    mockInterviewsResponse,
    selectedIsPassResult,
    selectedMockInterviewsPreset,
    selectedInterviewers,
  ]);

  const mockInterviewsPresetOptions = [
    {
      value: MockInterviewsPreset.ReceivedMocks,
      label: 'Received (I was interviewed)',
    },
    {
      value: MockInterviewsPreset.GivenMocks,
      label: 'Given (I was the interviewer)',
    },
    {
      value: MockInterviewsPreset.All,
      label: 'All',
    },
  ];

  return (
    <>
      <Flex justify="space-between">
        <Stack mb="lg" gap={4}>
          <Text size="sm" fw={500}>
            Show mock interviews
          </Text>
          <SegmentedControl
            data={mockInterviewsPresetOptions}
            value={selectedMockInterviewsPreset.toString()}
            onChange={(value) => {
              setSelectedMockInterviewsPreset(value as MockInterviewsPreset);
            }}
          />
        </Stack>
        <Group mb="lg" justify="flex-end">
          <MultiSelect
            classNames={{ inputField: classes.inputField }}
            label="Interviewer"
            placeholder="Pick value(s)"
            data={distinctInterviewersOptions}
            filter={createOptionsFilter()}
            miw={150}
            searchable
            nothingFoundMessage="Nothing found..."
            value={selectedInterviewers}
            onChange={(values) => {
              setSelectedInterviewers(values as string[]);
            }}
          />
          <Select
            label="Result"
            data={resultOptions}
            placeholder="Pick value"
            filter={createOptionsFilter()}
            miw={150}
            nothingFoundMessage="Nothing found..."
            value={selectedIsPassResult === null ? null : selectedIsPassResult.toString()}
            clearable
            onChange={(value) => {
              if (value === null || value === '') {
                setSelectedIsPassResult(null);
              } else {
                setSelectedIsPassResult(value === 'true');
              }
            }}
          />
        </Group>
      </Flex>
      <MockInterviewTable
        refetchMockInterviews={refetchMockInterviews}
        mockInterviews={filteredMockInterviews}
        seasonId={seasonId || ''}
        enableEditing
      />
    </>
  );
}
