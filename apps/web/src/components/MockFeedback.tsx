import { RichTextContent } from '@/components/RichTextContent';
import type { MockInterview } from '@/types';
import styles from '@/styles/App.module.css';

export function MockFeedback({ mock }: { mock: MockInterview }) {
  return (
    <div className={styles.panel}>
      <h3 className={styles.cardTitle}>Interview feedback</h3>
      <RichTextContent html={mock.notes} empty="No overall interview notes." />
      <div className={styles.rounds}>
        {mock.rounds.map((round) => (
          <article className={styles.round} key={round.id}>
            <h4>{round.title}</h4>
            {round.link ? (
              <a href={round.link} target="_blank" rel="noreferrer">
                Open question
              </a>
            ) : null}
            <dl className={styles.cleanList}>
              {Object.entries(round.scores).map(([name, score]) => (
                <div className={styles.listRow} key={name}>
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
            <p className={styles.helper}>
              {round.reviewed ? 'Reviewed by interviewee' : 'Review pending'}
            </p>
            <RichTextContent html={round.intervieweeComment} />
          </article>
        ))}
      </div>
    </div>
  );
}
