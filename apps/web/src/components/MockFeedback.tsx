import { Card } from '@/components/ui/card';
import { RichTextContent } from '@/components/RichTextContent';
import type { MockInterview } from '@/types';

export function MockFeedback({ mock }: { mock: MockInterview }) {
  return (
    <Card className="gap-4 p-5">
      <h3 className="text-base font-semibold tracking-tight">
        Interview feedback
      </h3>
      <RichTextContent html={mock.notes} empty="No overall interview notes." />
      <div className="grid">
        {mock.rounds.map((round) => (
          <article
            className="py-3 first:pt-0 [&+&]:border-t [&+&]:border-border"
            key={round.id}
          >
            <h4>{round.title}</h4>
            {round.link ? (
              <a href={round.link} target="_blank" rel="noreferrer">
                Open question
              </a>
            ) : null}
            <dl className="divide-y divide-border">
              {Object.entries(round.scores).map(([name, score]) => (
                <div
                  className="flex items-center justify-between gap-4 py-2"
                  key={name}
                >
                  <dt>
                    {name
                      .replaceAll(/([A-Z])/g, ' $1')
                      .replace(/^./, (letter) => letter.toUpperCase())}
                  </dt>
                  <dd>{score}/10</dd>
                </div>
              ))}
            </dl>
            <RichTextContent html={round.notes} />
            <p className="mt-1 text-sm text-muted-foreground">
              {round.reviewed ? 'Reviewed by interviewee' : 'Review pending'}
            </p>
            <RichTextContent html={round.intervieweeComment} />
          </article>
        ))}
      </div>
    </Card>
  );
}
