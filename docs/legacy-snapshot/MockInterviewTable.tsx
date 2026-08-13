import dayjs from 'dayjs';
import { useMemo, useState } from 'react';
import { IconEdit, IconExternalLink, IconInfoCircle, IconTrash } from '@tabler/icons-react';
import { QueryObserverResult, RefetchOptions } from '@tanstack/react-query';
import {
  MantineReactTable,
  MRT_ColumnDef,
  MRT_Row,
  useMantineReactTable,
} from 'mantine-react-table';
import {
  ActionIcon,
  Anchor,
  Box,
  Button,
  Flex,
  Switch,
  Table,
  Text,
  Title,
  Tooltip,
} from '@mantine/core';
import { useLocalStorage } from '@mantine/hooks';
import { modals } from '@mantine/modals';
import { notifications } from '@mantine/notifications';
import { ProfileLink } from '@/components/ProfileLink/ProfileLink';
import {
  DeleteMockInterviewResponseApiResponse,
  ListMockInterviewResponseApiResponse,
  MockInterviewEntity,
  MockInterviewRoundEntity,
  UpdateMockInterviewRoundReviewResponseApiResponse,
  useCreateMockInterview,
  useDeleteMockInterview,
  useGetCurrentUser,
  useGetEnrollmentUsers,
  useListLeetcodeProblems,
  useUpdateCustomMockInterviewRoundReview,
  useUpdateLeetcodeMockInterviewRoundReview,
  useUpdateMockInterview,
} from '@/generated/api/client';
import { LeetcodeDifficultyText } from '@/shared/components/LeetcodeDifficultyText';
import {
  getConfirmModalProps,
  getErrorNotification,
  getMantineTablePropsWithBanner,
  getSuccessNotification,
  NOTIFICATION_MESSAGES,
} from '@/shared/constants/mantineTableProps';
import { CONFIRMATION_MESSAGES } from '@/shared/constants/messages';
import classes from '@/shared/styles/tableStyles.module.css';
import { MockInterviewScoreColors } from '@/shared/utils/colorMappings';
import { MockInterviewCreateModal } from './MockInterviewCreateModal';
import { MockInterviewUpdateModal } from './MockInterviewUpdateModal';

export const MockInterviewTable = ({
  refetchMockInterviews,
  seasonId,
  mockInterviews,
  enableEditing,
}: MockInterviewTableProps) => {
  const [pageSize, setPageSize] = useLocalStorage({
    key: 'page-size',
    defaultValue: 10,
    getInitialValueInEffect: false,
  });

  const [pagination, setPagination] = useState({
    pageIndex: 0,
    pageSize,
  });

  const {
    data: leetcodeProblemsResponse,
    isError: isLoadingLeetcodeProblemsError,
    isFetching: isFetchingLeetcodeProblems,
    isLoading: isLoadingLeetcodeProblems,
  } = useListLeetcodeProblems();

  const {
    data: usersResponse,
    isError: isLoadingUsersError,
    isFetching: isFetchingUsers,
    isLoading: isLoadingUsers,
  } = useGetEnrollmentUsers();
  const { data: currentUserResponse } = useGetCurrentUser();
  const currentUserId = currentUserResponse?.responseBody?.user.userId ?? '';
  const users = usersResponse?.responseBody?.enrollmentUsers.filter(
    (u) => u.userId !== currentUserId
  );

  const { mutateAsync: createMockInterview, status: isCreatingMockInterviewStatus } =
    useCreateMockInterview();
  const { mutateAsync: updateMockInterview, status: isUpdatingMockInterviewStatus } =
    useUpdateMockInterview();
  const { mutateAsync: deleteMockInterview, status: isDeletingMockInterviewStatus } =
    useDeleteMockInterview();

  const openDeleteConfirmModal = (row: MRT_Row<MockInterviewEntity>) => {
    modals.openConfirmModal({
      children: (
        <>
          <Title order={3} mt={15} mb={10}>
            {CONFIRMATION_MESSAGES.MOCK_INTERVIEW.DELETE_TITLE}
          </Title>
          <Text>{CONFIRMATION_MESSAGES.MOCK_INTERVIEW.DELETE_TEXT}</Text>
        </>
      ),
      labels: { confirm: 'Delete', cancel: 'Cancel' },
      ...getConfirmModalProps(),
      onConfirm: async () => {
        try {
          await deleteMockInterview({
            data: { mockInterviewId: row.original.mockInterviewId, userId: currentUserId },
          });
          await refetchMockInterviews();
          modals.closeAll();
          notifications.show(getSuccessNotification(NOTIFICATION_MESSAGES.MOCK_INTERVIEW.DELETED));
        } catch (err) {
          const response = (err as any)?.response.data as DeleteMockInterviewResponseApiResponse;
          notifications.show(getErrorNotification(response.error?.message));
        }
      },
    });
  };

  const enrollmentColumn: MRT_ColumnDef<MockInterviewEntity> | null =
    seasonId === null || seasonId === ''
      ? {
          header: 'Season',
          accessorFn: (row) => row.season?.slug || 'No Season',
        }
      : null;

  const seasonWeekColumn: MRT_ColumnDef<MockInterviewEntity> | null =
    seasonId != null && seasonId !== ''
      ? {
          header: 'Season Week',
          accessorFn: (row) => row.seasonWeek?.weekNumber || 'No Season week',
        }
      : null;

  const columns = useMemo<MRT_ColumnDef<MockInterviewEntity>[]>(
    () => [
      {
        header: 'Date',
        id: 'startDate',
        accessorFn: (row) => new Date(row.startDate),
        Cell: ({ row }) => {
          const startFormatted = dayjs(row.original.startDate).format('D MMM YYYY HH:mm');
          const isDataBackFilled = row.original.season?.isDataBackFilled;

          return (
            <Flex align="center" gap="xs">
              <Text size="sm">{startFormatted}</Text>
              {isDataBackFilled && (
                <Tooltip
                  label="This data was backfilled based on historical records"
                  position="top"
                >
                  <IconInfoCircle size={14} style={{ color: 'var(--mantine-color-blue-6)' }} />
                </Tooltip>
              )}
            </Flex>
          );
        },
      },
      ...(enrollmentColumn ? [enrollmentColumn] : []),
      ...(seasonWeekColumn ? [seasonWeekColumn] : []),
      {
        header: 'Duration',
        accessorFn: (row) => `${row.timeTakenInMinutes} mins`,
      },
      {
        header: 'Interviewer',
        accessorFn: (row) => row.interviewer?.name || 'Error',
        Cell: ({ row }) => {
          return (
            <ProfileLink
              userName={row.original.interviewer?.name}
              userSlug={row.original.interviewer?.slug}
            />
          );
        },
      },
      {
        header: 'Interviewee',
        accessorFn: (row) => (row.interviewee?.name ? `${row.interviewee.name}` : 'Error'),
        Cell: ({ row }) => {
          return (
            <ProfileLink
              userName={row.original.interviewee?.name}
              userSlug={row.original.interviewee?.slug}
            />
          );
        },
      },
      {
        header: 'Result',
        accessorFn: (row) => (row.isPass ? 'Pass' : 'Fail'),
        Cell: ({ row }) => {
          return (
            <Text size="sm" fw={500} c={row.original.isPass ? 'green.8' : 'red.8'}>
              {row.original.isPass ? 'Pass' : 'Fail'}
            </Text>
          );
        },
      },
      {
        header: 'Behavioural',
        accessorFn: (row) => {
          const rounds = row.mockInterviewRounds || [];
          const behaviouralRound = rounds.filter((r) => r.behaviouralMockInterviewRound != null)[0];
          return behaviouralRound?.behaviouralMockInterviewRound?.behavioralScore || 0;
        },
        Cell: ({ row }) => {
          const rounds = row.original.mockInterviewRounds || [];
          const behaviouralRound = rounds.filter((r) => r.behaviouralMockInterviewRound != null)[0];
          const score = behaviouralRound?.behaviouralMockInterviewRound?.behavioralScore || 0;
          return <ScoreText score={score} />;
        },
      },
    ],
    []
  );

  const table = useMantineReactTable({
    columns,
    data: mockInterviews ?? [],
    ...getMantineTablePropsWithBanner(
      classes.tableNoHover,
      isLoadingLeetcodeProblemsError || isLoadingUsersError
    ),
    createDisplayMode: 'modal',
    onPaginationChange: (updater) => {
      const next = typeof updater === 'function' ? updater(pagination) : updater;
      setPageSize(next.pageSize);
      setPagination(next);
    },
    editDisplayMode: 'modal',
    enableEditing,
    initialState: {
      density: 'xs',
      expanded: true,
      sorting: [
        {
          id: 'startDate',
          desc: true,
        },
      ],
    },
    positionActionsColumn: 'last',
    getRowId: (row) => row.mockInterviewId?.toString(),
    isMultiSortEvent: () => true,
    renderCreateRowModalContent: ({ table }) =>
      enableEditing && (
        <MockInterviewCreateModal
          table={table}
          users={users}
          leetcodeProblems={leetcodeProblemsResponse?.responseBody?.leetcodeProblems}
          seasonId={seasonId}
          createMockInterview={createMockInterview}
          refetchMockInterviews={refetchMockInterviews}
        />
      ),
    renderEditRowModalContent: ({ table, row }) =>
      enableEditing && (
        <MockInterviewUpdateModal
          table={table}
          row={row}
          users={users}
          leetcodeProblems={leetcodeProblemsResponse?.responseBody?.leetcodeProblems}
          updateMockInterview={updateMockInterview}
          refetchMockInterviews={refetchMockInterviews}
        />
      ),
    renderDetailPanel: ({ row }) => {
      const rounds = row.original.mockInterviewRounds || [];
      const leetcodeRounds = rounds.filter((r) => r.leetcodeMockInterviewRound != null);
      const customRounds = rounds.filter((r) => r.customMockInterviewRound != null);

      return (
        <Flex gap={20} direction="column">
          {leetcodeRounds.length > 0 && (
            <LeetcodeMockInterviewRoundsInnerTable
              rounds={leetcodeRounds}
              refetchMockInterviews={refetchMockInterviews}
            />
          )}
          {customRounds.length > 0 && (
            <CustomMockInterviewRoundsInnerTable
              rounds={customRounds}
              refetchMockInterviews={refetchMockInterviews}
            />
          )}
        </Flex>
      );
    },
    renderRowActions: ({ row, table }) =>
      enableEditing && (
        <Flex gap="md">
          <Tooltip label="Edit">
            <ActionIcon variant="subtle" onClick={() => table.setEditingRow(row)}>
              <IconEdit />
            </ActionIcon>
          </Tooltip>
          <Tooltip label="Delete">
            <ActionIcon variant="subtle" color="red" onClick={() => openDeleteConfirmModal(row)}>
              <IconTrash />
            </ActionIcon>
          </Tooltip>
        </Flex>
      ),
    renderTopToolbarCustomActions: ({ table }) =>
      enableEditing && (
        <Button
          onClick={() => {
            table.setCreatingRow(true);
          }}
        >
          Create Mock Interview
        </Button>
      ),
    state: {
      pagination,
      isLoading: isLoadingLeetcodeProblems || isLoadingUsers,
      isSaving:
        isCreatingMockInterviewStatus === 'pending' ||
        isUpdatingMockInterviewStatus === 'pending' ||
        isDeletingMockInterviewStatus === 'pending',
      showAlertBanner: isLoadingLeetcodeProblemsError || isLoadingUsersError,
      showProgressBars: isFetchingLeetcodeProblems || isFetchingUsers,
    },
  });

  return <MantineReactTable table={table} />;
};

type MockInterviewTableProps = {
  refetchMockInterviews: (
    options?: RefetchOptions
  ) => Promise<QueryObserverResult<ListMockInterviewResponseApiResponse, unknown>>;
  mockInterviews: MockInterviewEntity[] | null | undefined;
  seasonId: string;
  enableEditing: boolean;
};

const LeetcodeMockInterviewRoundsInnerTable = ({
  rounds,
  refetchMockInterviews,
}: InnerMockInterviewTableProps) => {
  const { mutateAsync: updateLeetcodeReview } = useUpdateLeetcodeMockInterviewRoundReview();

  const handleReviewToggle = async (roundId: string, newIsReviewed: boolean) => {
    try {
      await updateLeetcodeReview({
        data: {
          leetcodeMockInterviewRoundId: roundId,
          isReviewed: newIsReviewed,
        },
      });
      await refetchMockInterviews();
      notifications.show(
        getSuccessNotification(NOTIFICATION_MESSAGES.MOCK_INTERVIEW.REVIEW_UPDATED)
      );
    } catch (err) {
      const response = (err as any)?.response
        .data as UpdateMockInterviewRoundReviewResponseApiResponse;
      notifications.show(getErrorNotification(response.error?.message));
    }
  };

  const tableRows = rounds.map((r, index) => {
    if (r.leetcodeMockInterviewRound == null) {
      return null;
    }

    const leetcodeMock = r.leetcodeMockInterviewRound;

    return (
      <Table.Tr key={index}>
        <Table.Td>
          <Anchor
            href={leetcodeMock.leetcodeProblem?.problem?.link}
            target="_blank"
            inherit
            className={classes.title}
            underline="always"
          >
            <Flex align="center" gap="xs">
              <Text size="sm">{leetcodeMock.leetcodeProblem?.problem?.title || ''}</Text>
              <IconExternalLink size={12} style={{ flexShrink: 0, opacity: 0.7 }} />
            </Flex>
          </Anchor>
        </Table.Td>
        <Table.Td>
          <LeetcodeDifficultyText
            difficulty={leetcodeMock.leetcodeProblem?.leetcodeProblemDifficulty}
          />
        </Table.Td>
        <Table.Td>
          <ScoreText score={leetcodeMock.confirmQuestionScore} />
        </Table.Td>
        <Table.Td>
          <ScoreText score={leetcodeMock.algorithmDesignScore} />
        </Table.Td>
        <Table.Td>
          <ScoreText score={leetcodeMock.complexityAnalysisScore} />
        </Table.Td>
        <Table.Td>
          <ScoreText score={leetcodeMock.codingScore} />
        </Table.Td>
        <Table.Td>
          <ScoreText score={leetcodeMock.testingScore} />
        </Table.Td>
        <Table.Td>
          <Switch
            checked={leetcodeMock.isReviewed === true}
            onChange={(event) =>
              handleReviewToggle(
                leetcodeMock.leetcodeMockInterviewRoundId,
                event.currentTarget.checked
              )
            }
            label="Reviewed"
            size="sm"
          />
        </Table.Td>
      </Table.Tr>
    );
  });

  if (rounds.length === 0) {
    return null;
  }

  return (
    <Table horizontalSpacing="md" verticalSpacing="sm" className={classes.innerTable}>
      <Table.Thead className={classes.innerTableHeading}>
        <Table.Tr>
          <Table.Th>Leetcode Problem</Table.Th>
          <Table.Th>Difficulty</Table.Th>
          <Table.Th>Confirm Question</Table.Th>
          <Table.Th>Algorithm Design</Table.Th>
          <Table.Th>Complexity Analysis</Table.Th>
          <Table.Th>Code</Table.Th>
          <Table.Th>Test</Table.Th>
          <Table.Th>Reviewed</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>{tableRows}</Table.Tbody>
    </Table>
  );
};

const CustomMockInterviewRoundsInnerTable = ({
  rounds,
  refetchMockInterviews,
}: InnerMockInterviewTableProps) => {
  const { mutateAsync: updateCustomReview } = useUpdateCustomMockInterviewRoundReview();

  const handleReviewToggle = async (roundId: string, newIsReviewed: boolean) => {
    try {
      await updateCustomReview({
        data: {
          customMockInterviewRoundId: roundId,
          isReviewed: newIsReviewed,
        },
      });
      await refetchMockInterviews();
      notifications.show(
        getSuccessNotification(NOTIFICATION_MESSAGES.MOCK_INTERVIEW.REVIEW_UPDATED)
      );
    } catch (err) {
      const response = (err as any)?.response
        .data as UpdateMockInterviewRoundReviewResponseApiResponse;
      notifications.show(getErrorNotification(response.error?.message));
    }
  };

  const tableRows = rounds.map((r, index) => {
    if (r.customMockInterviewRound == null) {
      return null;
    }

    const customMock = r.customMockInterviewRound;

    return (
      <Table.Tr key={index}>
        <Table.Td>
          <Box
            style={{ fontSize: '14px' }}
            dangerouslySetInnerHTML={{ __html: customMock.content || '' }}
          />
        </Table.Td>
        <Table.Td>
          <Anchor
            href={customMock.link || ''}
            target="_blank"
            size="sm"
            fw={500}
            style={{
              maxWidth: '200px',
              display: 'block',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
            title={customMock.link || ''}
          >
            {customMock.link}
          </Anchor>
        </Table.Td>
        <Table.Td>
          <ScoreText score={customMock.score} />
        </Table.Td>
        <Table.Td>
          <Switch
            checked={customMock.isReviewed}
            onChange={(event) =>
              handleReviewToggle(customMock.customMockInterviewRoundId, event.currentTarget.checked)
            }
            label="Reviewed"
            size="sm"
          />
        </Table.Td>
      </Table.Tr>
    );
  });

  if (rounds.length === 0) {
    return null;
  }

  return (
    <Table horizontalSpacing="md" verticalSpacing="sm" className={classes.innerTable}>
      <Table.Thead className={classes.innerTableHeading}>
        <Table.Tr>
          <Table.Th>Custom Problem Content</Table.Th>
          <Table.Th>Link</Table.Th>
          <Table.Th>Score</Table.Th>
          <Table.Th>Reviewed</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>{tableRows}</Table.Tbody>
    </Table>
  );
};

type InnerMockInterviewTableProps = {
  rounds: MockInterviewRoundEntity[];
  refetchMockInterviews: (
    options?: RefetchOptions
  ) => Promise<QueryObserverResult<ListMockInterviewResponseApiResponse, unknown>>;
};

const ScoreText = ({ score }: ScoreTextProps) => {
  return (
    <Flex align="center" gap={6}>
      <Box bg={MockInterviewScoreColors[score]} w={12} h={12} className={classes.scoreTextCircle} />
      <Text size="sm">{score}</Text>
    </Flex>
  );
};

type ScoreTextProps = {
  score: number;
};
