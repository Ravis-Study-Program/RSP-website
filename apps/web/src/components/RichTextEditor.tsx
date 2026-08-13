import Link from '@tiptap/extension-link';
import { EditorContent, useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import { IconBold, IconItalic, IconLink, IconList, IconListNumbers } from '@tabler/icons-react';
import { useEffect } from 'react';

import styles from '@/styles/App.module.css';

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
    extensions: [StarterKit, Link.configure({ openOnClick: false, autolink: true, defaultProtocol: 'https' })],
    content: value,
    editorProps: { attributes: { id, 'aria-labelledby': `${id}-label`, 'aria-describedby': error ? `${id}-error` : `${id}-help` } },
    onUpdate: ({ editor: activeEditor }) => onChange(activeEditor.getHTML()),
  });

  useEffect(() => {
    if (editor && editor.getHTML() !== value) editor.commands.setContent(value, { emitUpdate: false });
  }, [editor, value]);

  const setLink = () => {
    if (!editor) return;
    const current = editor.getAttributes('link').href as string | undefined;
    const href = window.prompt('Enter a link address', current ?? 'https://');
    if (href === null) return;
    if (!href.trim()) editor.chain().focus().unsetLink().run();
    else editor.chain().focus().extendMarkRange('link').setLink({ href }).run();
  };

  return (
    <div className={styles.field}>
      <span id={`${id}-label`} className={styles.fieldLabel}>{label}</span>
      <div className={styles.editor} aria-invalid={Boolean(error)}>
        <div className={styles.editorToolbar} role="toolbar" aria-label={`${label} formatting`}>
          <EditorButton label="Bold" active={editor?.isActive('bold')} onClick={() => editor?.chain().focus().toggleBold().run()}><IconBold size={18} /></EditorButton>
          <EditorButton label="Italic" active={editor?.isActive('italic')} onClick={() => editor?.chain().focus().toggleItalic().run()}><IconItalic size={18} /></EditorButton>
          <EditorButton label="Bulleted list" active={editor?.isActive('bulletList')} onClick={() => editor?.chain().focus().toggleBulletList().run()}><IconList size={18} /></EditorButton>
          <EditorButton label="Numbered list" active={editor?.isActive('orderedList')} onClick={() => editor?.chain().focus().toggleOrderedList().run()}><IconListNumbers size={18} /></EditorButton>
          <EditorButton label="Link" active={editor?.isActive('link')} onClick={setLink}><IconLink size={18} /></EditorButton>
        </div>
        <EditorContent className={styles.editorContent} editor={editor} />
      </div>
      <p id={`${id}-help`} className={styles.helper}>Use the toolbar for supported formatting. Unsafe markup is removed by the server.</p>
      {error ? <p id={`${id}-error`} className={styles.fieldError}>{error}</p> : null}
    </div>
  );
}

function EditorButton({ label, active = false, onClick, children }: { label: string; active?: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button className={styles.editorButton} type="button" aria-label={label} aria-pressed={active} onClick={onClick}>
      {children}
    </button>
  );
}
