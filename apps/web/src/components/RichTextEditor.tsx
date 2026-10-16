import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from '@/components/ui/dialog';
import {
  EditorContent,
  Extension,
  Mark,
  mergeAttributes,
  useEditor,
} from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import {
  IconAlignCenter,
  IconAlignJustified,
  IconAlignLeft,
  IconAlignRight,
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
    <div className="grid gap-1.5">
      <span id={`${id}-label`} className="text-sm font-medium">
        {label}
      </span>
      <div
        className="overflow-hidden rounded-lg border border-input bg-card focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50 aria-invalid:border-destructive"
        aria-invalid={Boolean(error)}
      >
        <div
          className="flex flex-wrap items-center gap-1 border-b bg-muted p-1.5"
          role="toolbar"
          aria-label={`${label} formatting`}
        >
          <div className="flex shrink-0 items-center gap-0.5">
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
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <Separator
              orientation="vertical"
              className="mx-1 h-5 self-center"
            />
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
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <Separator
              orientation="vertical"
              className="mx-1 h-5 self-center"
            />
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
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <Separator
              orientation="vertical"
              className="mx-1 h-5 self-center"
            />
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
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <Separator
              orientation="vertical"
              className="mx-1 h-5 self-center"
            />
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
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <Separator
              orientation="vertical"
              className="mx-1 h-5 self-center"
            />
            {(
              [
                ['left', IconAlignLeft],
                ['center', IconAlignCenter],
                ['right', IconAlignRight],
                ['justify', IconAlignJustified],
              ] as const
            ).map(([alignment, Icon]) => (
              <EditorButton
                key={alignment}
                label={`Align ${alignment}`}
                active={editor?.isActive({ textAlign: alignment })}
                onClick={() => setAlignment(alignment)}
              >
                <Icon size={18} aria-hidden="true" />
              </EditorButton>
            ))}
          </div>
          <div className="flex shrink-0 items-center gap-0.5">
            <Separator
              orientation="vertical"
              className="mx-1 h-5 self-center"
            />
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
        </div>
        <EditorContent
          className="[&_.tiptap]:min-h-32 [&_.tiptap]:p-3 [&_.tiptap]:outline-none [&_.tiptap_p:first-child]:mt-0 [&_[data-text-align=center]]:text-center [&_[data-text-align=right]]:text-right [&_[data-text-align=justify]]:text-justify"
          editor={editor}
        />
      </div>
      <Dialog
        open={linkOpen}
        onOpenChange={(next) => {
          setLinkOpen(next);
          if (!next) {
            setLinkError('');
            window.requestAnimationFrame(() => linkButtonRef.current?.focus());
          }
        }}
      >
        <DialogContent showCloseButton={false}>
          <DialogTitle>Add or edit link</DialogTitle>
          <DialogDescription>
            Links may use HTTP, HTTPS or mailto addresses.
          </DialogDescription>
          <form className="grid gap-4" onSubmit={saveLink}>
            <div className="grid gap-1.5">
              <label
                className="text-sm font-medium"
                htmlFor={`${id}-link-address`}
              >
                Link address
              </label>
              <Input
                id={`${id}-link-address`}
                type="text"
                inputMode="url"
                autoComplete="url"
                // The link popover is a dialog; moving focus to its only
                // text field is the expected keyboard interaction.
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                value={linkAddress}
                aria-invalid={Boolean(linkError)}
                aria-describedby={linkError ? `${id}-link-error` : undefined}
                onChange={(event) => {
                  setLinkAddress(event.target.value);
                  setLinkError('');
                }}
              />
              {linkError ? (
                <p
                  id={`${id}-link-error`}
                  className="text-sm text-destructive"
                  role="alert"
                >
                  {linkError}
                </p>
              ) : null}
            </div>
            <div className="mt-5 flex justify-end gap-2.5">
              <DialogClose render={<Button variant="outline" />}>
                Cancel
              </DialogClose>
              <Button type="submit">Save link</Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
      <p id={`${id}-help`} className="text-sm text-muted-foreground">
        Use the toolbar for supported formatting. Unsafe markup is removed by
        the server.
      </p>
      {error ? (
        <p id={`${id}-error`} className="text-sm text-destructive">
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
    <Button
      variant="ghost"
      size="icon"
      ref={buttonRef}
      className="h-8 w-auto min-w-8 px-1.5 aria-pressed:bg-primary aria-pressed:text-primary-foreground"
      type="button"
      aria-label={label}
      aria-pressed={active}
      onClick={onClick}
    >
      {children}
    </Button>
  );
}
