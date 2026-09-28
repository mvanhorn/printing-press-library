#!/usr/bin/env python3
"""Research-only Nuxt/devalue decoder; reads local public SSR captures."""
import argparse, json, re
from pathlib import Path

def decode_capture(path):
    html = Path(path).read_text()
    match = re.search(r'<script\b[^>]*\bid="__NUXT_DATA__"[^>]*>(.*?)</script>', html, re.S)
    if not match:
        raise ValueError('No __NUXT_DATA__ script found')
    values = json.loads(match.group(1))
    memo = {}
    def ref(index):
        if index < 0:
            return {-1: {'$undefined': True}, -2: {'$hole': True}, -3: {'$NaN': True}, -4: {'$Infinity': True}, -5: {'$negativeInfinity': True}, -6: {'$negativeZero': True}}[index]
        if index in memo:
            return memo[index]
        value = values[index]
        if isinstance(value, dict):
            result = {}
            memo[index] = result
            result.update({k: ref(v) for k, v in value.items()})
        elif isinstance(value, list):
            if value and isinstance(value[0], str):
                kind = value[0]
                if kind in ('Reactive', 'ShallowReactive', 'Ref', 'ShallowRef'):
                    result = ref(value[1])
                elif kind == 'Set':
                    result = [ref(v) for v in value[1:]]
                elif kind == 'Map':
                    result = {'$Map': [[ref(value[i]), ref(value[i+1])] for i in range(1,len(value),2)]}
                elif kind in ('Date', 'BigInt', 'RegExp', 'URL'):
                    result = {'$'+kind: value[1:]}
                elif kind == 'null':
                    result = {str(value[i]): ref(value[i+1]) for i in range(1,len(value),2)}
                else:
                    result = {'$type': kind, '$values': [ref(v) if isinstance(v, int) else v for v in value[1:]]}
                memo[index] = result
            else:
                result = []
                memo[index] = result
                result.extend(ref(v) for v in value)
        else:
            result = value
            memo[index] = result
        return result
    return html, ref(0)

def operations(root):
    for key, value in root.get('data', {}).items():
        try:
            op = json.loads(key[:key.rfind('}')+1])
        except (ValueError, TypeError):
            continue
        yield op.get('o'), op.get('v', {}), value, key

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('capture')
    parser.add_argument('--operation')
    parser.add_argument('--links', action='store_true')
    args=parser.parse_args()
    html,root=decode_capture(args.capture)
    if args.links:
        links=sorted(set(re.findall(r'href="([^"]+)"',html)))
        print(json.dumps(links,ensure_ascii=False,indent=2))
    elif args.operation:
        print(json.dumps([{'operation':o,'variables':v,'value':x} for o,v,x,k in operations(root) if o==args.operation],ensure_ascii=False,indent=2))
    else:
        print(json.dumps([{'operation':o,'variables':v,'top_keys':list(x) if isinstance(x,dict) else None,'skip':k.endswith('-skip')} for o,v,x,k in operations(root)],ensure_ascii=False,indent=2))
if __name__ == '__main__':
    main()
