"""Package a standalone GHCR installation kit from an explicit file allowlist."""
import argparse
from pathlib import Path
import re
import zipfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--tag', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'sha-[0-9a-f]{40}', args.tag):
        parser.error('--tag must be sha- followed by a full 40-character commit ID')
    source = Path(__file__).resolve().parent
    repo = source.parent.parent
    files = {name: (source / name).read_bytes()
             for name in ('compose.yaml', 'prepare.sh', 'init-db.sh', 'README.md')}
    compose = files['compose.yaml'].decode()
    old_mount = '../helmchart/files/sql:/sql:ro'
    if compose.count(old_mount) != 1:
        raise ValueError('Expected exactly one SQL mount in compose.yaml')
    files['compose.yaml'] = compose.replace(old_mount, './sql:/sql:ro').encode()
    files['release.env'] = f'WATCHALERT_RELEASE_TAG={args.tag}\n'.encode()
    for name in ('notice_template_examples.sql', 'rule_template_groups.sql',
                 'rule_templates.sql', 'tenants.sql', 'tenants_linked_users.sql'):
        files['sql/' + name] = (repo / 'deploy/helmchart/files/sql' / name).read_bytes()
    files['LICENSE'] = (repo / 'LICENSE').read_bytes()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(args.output, 'x', compression=zipfile.ZIP_DEFLATED) as archive:
        for name, content in files.items():
            info = zipfile.ZipInfo('watchalert-ghcr/' + name)
            info.external_attr = (0o100755 if name.endswith('.sh') else 0o100644) << 16
            archive.writestr(info, content, compress_type=zipfile.ZIP_DEFLATED)
    print(f'Created {args.output} with {len(files)} files; no runtime secrets or data included.')


if __name__ == '__main__':
    main()
