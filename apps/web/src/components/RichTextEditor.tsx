import { Dialog } from '@base-ui/react/dialog';
import {
  EditorContent,
  Extension,
  Mark,
  mergeAttributes,
  useEditor,
} from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import {
  IconArrowBackUp,
  IconArrowForwardUp,
  IconBold,
  IconClearFormatting,
  IconCode,
  IconHighlight,
  IconItalic,
  IconLink,
  IconList,
  IconListNumbers,
  IconMinus,
  IconStrikethrough,
  IconUnderline,
  IconUnlink,
} from '@tabler/icons-react';
import { useEffect, useRef, useState } from 'react';

import styles from '@/styles/App.module.css';

function simpleMark(name: string, tag: string) {
  return Mark.create({
    name,
    parseHTML: () => [{ tag }],
    renderHTML: ({ HTMLAttributes }) => [
      tag,
      mergeAttributes(HTMLAttributes),
      0,
    ],
  });
}

const Highlight = simpleMark('highlight', 'mark');
const Subscript = simpleMark('subscript', 'sub');
const Superscript = simpleMark('superscript', 'sup');
const SafeTextAlign = Extension.create({
  name: 'safeTextAlign',
  addGlobalAttributes() {
    return [
      {
        types: ['paragraph', 'heading', 'blockquote'],
        attributes: {
          textAlign: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-text-align'),
            renderHTML: (attributes) =>
              ['left', 'center', 'right', 'justify'].includes(
                attributes.textAlign as string,
              )
                ? { 'data-text-align': attributes.textAlign }
                : {},
          },
        },
      },
    ];
  },
});

export function RichTextEditor({
  id,
  label,
  value,
  onChange,
  error,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3, 4] },
        link: { openOnClick: false, autolink: true, defaultProtocol: 'https' },
      }),
      Highlight,
      Subscript,
      Superscript,
      SafeTextAlign,
    ],
    content: value,
    editorProps: {
      attributes: {
        id,
        'aria-labelledby': `${id}-label`,
        'aria-describedby': error ? `${id}-error` : `${id}-help`,
      },
    },
    onUpdate: ({ editor: activeEditor }) => onChange(activeEditor.getHTML()),
  });

  useEffect(() => {
    if (editor && editor.getHTML() !== value)
      editor.commands.setContent(value, { emitUpdate: false });
  }, [editor, value]);

  const [linkOpen, setLinkOpen] = useState(false);
  const [linkAddress, setLinkAddress] = useState('https://');
  const [linkError, setLinkError] = useState('');
  const linkButtonRef = useRef<HTMLButtonElement>(null);
  const openLinkDialog = () => {
    if (!editor) return;
    const current = editor.getAttributes('link').href as string | undefined;
    setLinkAddress(current ?? 'https://');
    setLinkError('');
    setLinkOpen(true);
  };
  const saveLink = (event: React.FormEvent) => {
    event.preventDefault();
    if (!editor) return;
    const href = normaliseSafeLink(linkAddress);
    if (!href) {
      setLinkError('Enter an HTTP, HTTPS or mailto address.');
      return;
    }
    editor.chain().extendMarkRange('link').setLink({ href }).run();
    setLinkOpen(false);
    window.requestAnimationFrame(() => linkButtonRef.current?.focus());
  };
  const setAlignment = (textAlign: 'left' | 'center' | 'right' | 'justify') =>
    editor
      ?.chain()
      .focus()
      .command(({ tr, state }) => {
        const { from, to } = state.selection;
        state.doc.nodesBetween(from, to, (node, position) => {
          if (['paragraph', 'heading', 'blockquote'].includes(node.type.name))
            tr.setNodeMarkup(position, undefined, { ...node.attrs, textAlign });
        });
        return true;
      })
      .run();

  return (
    <div className={styles.field}>
      <span id={`${id}-label`} className={styles.fieldLabel}>
        {label}
      </span>
      <div className={styles.editor} aria-invalid={Boolean(error)}>
        <div
          className={styles.editorToolbar}
          role="toolbar"
          aria-label={`${label} formatting`}
        >
          <EditorButton
            label="Bold"
            active={editor?.isActive('bold')}
            onClick={() => editor?.chain().focus().toggleBold().run()}
          >
            <IconBold size={18} />
          </EditorButton>
          <EditorButton
            label="Italic"
            active={editor?.isActive('italic')}
            onClick={() => editor?.chain().focus().toggleItalic().run()}
          >
            <IconItalic size={18} />
          </EditorButton>
          <EditorButton
            label="Underline"
            active={editor?.isActive('underline')}
            onClick={() =>
              editor?.chain().focus().toggleMark('underline').run()
            }
          >
            <IconUnderline size={18} />
          </EditorButton>
          <EditorButton
            label="Strikethrough"
            active={editor?.isActive('strike')}
            onClick={() => editor?.chain().focus().toggleStrike().run()}
          >
            <IconStrikethrough size={18} />
          </EditorButton>
          <EditorButton
            label="Highlight"
            active={editor?.isActive('highlight')}
            onClick={() =>
              editor?.chain().focus().toggleMark('highlight').run()
            }
          >
            <IconHighlight size={18} />
          </EditorButton>
          <EditorButton
            label="Inline code"
            active={editor?.isActive('code')}
            onClick={() => editor?.chain().focus().toggleCode().run()}
          >
            <IconCode size={18} />
          </EditorButton>
          {[1, 2, 3, 4].map((level) => (
            <EditorButton
              key={level}
              label={`Heading ${level}`}
              active={editor?.isActive('heading', { level })}
              onClick={() =>
                editor
                  ?.chain()
                  .focus()
                  .toggleHeading({ level: level as 1 | 2 | 3 | 4 })
                  .run()
              }
            >
              H{level}
            </EditorButton>
          ))}
          <EditorButton
            label="Block quote"
            active={editor?.isActive('blockquote')}
            onClick={() => editor?.chain().focus().toggleBlockquote().run()}
          >
            Quote
          </EditorButton>
          <EditorButton
            label="Bulleted list"
            active={editor?.isActive('bulletList')}
            onClick={() => editor?.chain().focus().toggleBulletList().run()}
          >
            <IconList size={18} />
          </EditorButton>
          <EditorButton
            label="Numbered list"
            active={editor?.isActive('orderedList')}
            onClick={() => editor?.chain().focus().toggleOrderedList().run()}
          >
            <IconListNumbers size={18} />
          </EditorButton>
          <EditorButton
            label="Subscript"
            active={editor?.isActive('subscript')}
            onClick={() =>
              editor
                ?.chain()
                .focus()
                .unsetMark('superscript')
                .toggleMark('subscript')
                .run()
            }
          >
            X₂
          </EditorButton>
          <EditorButton
            label="Superscript"
            active={editor?.isActive('superscript')}
            onClick={() =>
              editor
                ?.chain()
                .focus()
                .unsetMark('subscript')
                .toggleMark('superscript')
                .run()
            }
          >
            X²
          </EditorButton>
          <EditorButton
            buttonRef={linkButtonRef}
            label="Link"
            active={editor?.isActive('link')}
            onClick={openLinkDialog}
          >
            <IconLink size={18} />
          </EditorButton>
          <EditorButton
            label="Remove link"
            onClick={() => editor?.chain().focus().unsetLink().run()}
          >
            <IconUnlink size={18} />
          </EditorButton>
          <EditorButton
            label="Horizontal rule"
            onClick={() => editor?.chain().focus().setHorizontalRule().run()}
          >
            <IconMinus size={18} />
          </EditorButton>
          {(['left', 'center', 'right', 'justify'] as const).map(
            (alignment) => (
              <EditorButton
                key={alignment}
                label={`Align ${alignment}`}
                active={editor?.isActive({ textAlign: alignment })}
                onClick={() => setAlignment(alignment)}
              >
                {alignment === 'justify'
                  ? 'Justify'
                  : alignment[0].toUpperCase()}
              </EditorButton>
            ),
          )}
          <EditorButton
            label="Clear formatting"
            onClick={() =>
              editor?.chain().focus().clearNodes().unsetAllMarks().run()
            }
          >
            <IconClearFormatting size={18} />
          </EditorButton>
          <EditorButton
            label="Undo"
            onClick={() => editor?.chain().focus().undo().run()}
          >
            <IconArrowBackUp size={18} />
          </EditorButton>
          <EditorButton
            label="Redo"
            onClick={() => editor?.chain().focus().redo().run()}
          >
            <IconArrowForwardUp size={18} />
          </EditorButton>
        </div>
        <EditorContent className={styles.editorContent} editor={editor} />
      </div>
      <Dialog.Root
        open={linkOpen}
        onOpenChange={(next) => {
          setLinkOpen(next);
          if (!next) {
            setLinkError('');
            window.requestAnimationFrame(() => linkButtonRef.current?.focus());
          }
        }}
      >
        <Dialog.Portal>
          <Dialog.Backdrop className={styles.dialogBackdrop} />
          <Dialog.Viewport className={styles.dialogViewport}>
            <Dialog.Popup className={styles.dialogPopup}>
              <Dialog.Title className={styles.dialogTitle}>
                Add or edit link
              </Dialog.Title>
              <Dialog.Description className={styles.dialogDescription}>
                Links may use HTTP, HTTPS or mailto addresses.
              </Dialog.Description>
              <form className={styles.form} onSubmit={saveLink}>
                <div className={styles.field}>
                  <label htmlFor={`${id}-link-address`}>Link address</label>
                  <input
                    id={`${id}-link-address`}
                    className={styles.input}
                    type="text"
                    inputMode="url"
                    autoComplete="url"
                    // The link popover is a dialog; moving focus to its only
                    // text field is the expected keyboard interaction.
                    // eslint-disable-next-line jsx-a11y/no-autofocus
                    autoFocus
                    value={linkAddress}
                    aria-invalid={Boolean(linkError)}
                    aria-describedby={
                      linkError ? `${id}-link-error` : undefined
                    }
                    onChange={(event) => {
                      setLinkAddress(event.target.value);
                      setLinkError('');
                    }}
                  />
                  {linkError ? (
                    <p
                      id={`${id}-link-error`}
                      className={styles.fieldError}
                      role="alert"
                    >
                      {linkError}
                    </p>
                  ) : null}
                </div>
                <div className={styles.dialogActions}>
                  <Dialog.Close className={styles.buttonSecondary}>
                    Cancel
                  </Dialog.Close>
                  <button className={styles.button} type="submit">
                    Save link
                  </button>
                </div>
              </form>
            </Dialog.Popup>
          </Dialog.Viewport>
        </Dialog.Portal>
      </Dialog.Root>
      <p id={`${id}-help`} className={styles.helper}>
        Use the toolbar for supported formatting. Unsafe markup is removed by
        the server.
      </p>
      {error ? (
        <p id={`${id}-error`} className={styles.fieldError}>
          {error}
        </p>
      ) : null}
    </div>
  );
}

function normaliseSafeLink(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return null;
  const candidate = /^[a-z][a-z\d+.-]*:/i.test(trimmed)
    ? trimmed
    : `https://${trimmed}`;
  try {
    const parsed = new URL(candidate);
    return ['http:', 'https:', 'mailto:'].includes(parsed.protocol)
      ? candidate
      : null;
  } catch {
    return null;
  }
}

function EditorButton({
  buttonRef,
  label,
  active = false,
  onClick,
  children,
}: {
  buttonRef?: React.Ref<HTMLButtonElement>;
  label: string;
  active?: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      ref={buttonRef}
      className={styles.editorButton}
      type="button"
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
