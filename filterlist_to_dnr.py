import json
import urllib.request
import re
import sys

def download_filterlist():
    print("Downloading FilterList...")
    url = "https://easylist.to/easylist/easylist.txt"
    req = urllib.request.Request(url, headers={'User-Agent': 'Mozilla/5.0'})
    with urllib.request.urlopen(req) as response:
        return response.read().decode('utf-8').splitlines()

def convert_to_dnr(lines):
    print("Converting FilterList to DeclarativeNetRequest rules...")
    rules = []
    rule_id = 1
    
    # Simple regex to match basic network block rules: ||example.com^
    # This is a very simplified parser, but it captures the most important tracker/ad domains.
    domain_block_regex = re.compile(r'^\|\|([a-zA-Z0-9.-]+)\^(\$([^,]+))?$')
    
    for line in lines:
        line = line.strip()
        if not line or line.startswith('!') or line.startswith('['):
            continue
            
        match = domain_block_regex.match(line)
        if match:
            domain = match.group(1)
            
            # Create a DNR rule for this domain
            rule = {
                "id": rule_id,
                "priority": 1,
                "action": { "type": "block" },
                "condition": {
                    "urlFilter": f"||{domain}^",
                    "resourceTypes": ["main_frame", "sub_frame", "script", "image", "xmlhttprequest", "ping", "media", "websocket"]
                }
            }
            rules.append(rule)
            rule_id += 1
            
            # Chrome DNR limits static rules to 30,000 per ruleset usually, 
            # and up to 330,000 total across all enabled rulesets.
            # We'll cap at 29,000 for safety in a single ruleset.
            if rule_id >= 29000:
                break
                
    return rules

def main():
    lines = download_filterlist()
    rules = convert_to_dnr(lines)
    
    output_file = '/home/cyber/CODES/aurablock/extension/rules.json'
    with open(output_file, 'w') as f:
        json.dump(rules, f, indent=2)
        
    print(f"Successfully generated {len(rules)} rules in {output_file}")

if __name__ == "__main__":
    main()
