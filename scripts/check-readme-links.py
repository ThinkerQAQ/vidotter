#!/usr/bin/env python3
"""Validate the two public READMEs without a network dependency.

CI checks relative links, image assets, anchors, and the common six-section
structure. External sites are checked separately during release review.
"""
from pathlib import Path
from urllib.parse import unquote, urlsplit
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
FILES = [ROOT / 'README.md', ROOT / 'README_ZH.md']
TOPICS = ['Introduction', 'Quick Start', 'Configuration', 'Optional Features',
          'Development', 'License']
LINK = re.compile(r'!?\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]+")?\)')
HEADER = re.compile(r'^#{1,6}\s+(.+?)\s*$', re.MULTILINE)


def slug(text: str) -> str:
    text = re.sub(r'[*_~\[\]()]', '', text.lower())
    text = re.sub(r'[^\w\- ]', '', text)
    return re.sub(r'\s+', '-', text.strip())


def validate(path: Path) -> list[str]:
    failures: list[str] = []
    if not path.is_file():
        return [f'Missing README: {path}']
    source = path.read_text(encoding='utf-8')
    found_topics = re.findall(r'^##\s+(.+)\s*$', source, flags=re.MULTILINE)
    if len(found_topics) != len(TOPICS) or any(
        not actual.startswith(expected)
        for actual, expected in zip(found_topics, TOPICS)
    ):
        failures.append(f'{path.name}: sections differ from {TOPICS}: {found_topics}')
    headings = {slug(h) for h in HEADER.findall(source)}
    for match in LINK.finditer(source):
        value = match.group(1).strip('<>')
        if value.startswith(('https://', 'http://', 'mailto:', 'data:')):
            continue
        parsed = urlsplit(value)
        if parsed.scheme or parsed.netloc:
            failures.append(f'{path.name}: unsupported relative URI: {value}')
            continue
        target = (path.parent / unquote(parsed.path)).resolve() if parsed.path else path
        if not target.is_relative_to(ROOT):
            failures.append(f'{path.name}: escapes repository: {value}')
        elif not target.exists():
            failures.append(f'{path.name}: broken link: {value}')
        elif parsed.fragment and target.suffix.lower() == '.md':
            if target == path:
                destination_headings = headings
            else:
                destination_headings = {slug(h) for h in HEADER.findall(
                    target.read_text(encoding='utf-8'))}
            if unquote(parsed.fragment).lower() not in destination_headings:
                failures.append(f'{path.name}: missing heading anchor: {value}')
    for check in ['assets/first-run-detect-zh.png',
                  'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'AGENTS.md']:
        if not (ROOT / check).exists():
            failures.append(f'Missing retained asset: {check}')
    return failures


def main() -> int:
    failures: list[str] = []
    for path in FILES:
        failures += validate(path)
    if (ROOT / 'docs').exists():
        failures.append('Old docs/ directory remains')
    for extra in (ROOT / 'build').rglob('README.md'):
        failures.append(f'Unexpected build README: {extra}')
    if (ROOT / 'mobile' / 'android' / 'README.md').exists():
        failures.append('Unexpected Android README')
    if failures:
        for problem in failures:
            print('README-Only FAILED:', problem, file=sys.stderr)
        return 1
    print('README-Only OK: 2 languages, 6 aligned sections, local links, '
          'retained image/legal assets, no docs tree')
    return 0


if __name__ == '__main__':
    sys.exit(main())
