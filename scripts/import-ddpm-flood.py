"""Refresh the embedded DDPM district history and separate 2568 provincial context.

Requires openpyxl. Raw downloads stay outside the repository; only aggregated
public statistics and their provenance are written to the API data directory.
The annual GD027 workbook is provincial: its District column contains counts,
not names, and must never extend the district-history period.
"""
import argparse
import csv
import hashlib
import io
import json
import math
import pathlib
import urllib.request
from datetime import datetime, timezone

import openpyxl

API = 'https://catalog.disaster.go.th/api/3/action/package_show?id='
OUTPUT = pathlib.Path(__file__).resolve().parents[1] / 'apps/api/internal/provider/data/ddpm_flood_snapshot.json'


def download(url):
    request = urllib.request.Request(url, headers={'User-Agent': 'RBC-EV-Station-data-import/1.0'})
    with urllib.request.urlopen(request, timeout=60) as response:
        payload = response.read(12 * 1024 * 1024)
    if len(payload) >= 12 * 1024 * 1024:
        raise ValueError('Source exceeds download limit')
    return payload


def package(identifier):
    result = json.loads(download(API + identifier))['result']
    if result['license_id'] != 'Open Data Common':
        raise ValueError('Review changed DDPM licence before importing')
    return result


def count(value):
    if value is None or str(value).strip() in ('', '-'):
        return 0
    number = float(str(value).replace(',', '').strip())
    if not math.isfinite(number) or number < 0 or not number.is_integer():
        raise ValueError(f'Invalid count: {value!r}')
    return int(number)


def provenance(resource, payload):
    return {'url': resource['url'], 'resourceId': resource['id'],
            'sha256': hashlib.sha256(payload).hexdigest(),
            'sourceUpdatedAt': resource.get('last_modified') or resource.get('metadata_modified')}


def build_snapshot():
    history = package('26674dbc-d656-4f4f-8afd-5a62c5b5e5b0')
    resources = [r for r in history['resources'] if r['format'].upper() == 'CSV']
    if len(resources) != 2:
        raise ValueError('Expected both district-history CSV parts; review source changes')
    areas, seen, sources = {}, set(), []
    for resource in resources:
        payload = download(resource['url'])
        sources.append(provenance(resource, payload))
        reader = csv.DictReader(io.StringIO(payload.decode('utf-8-sig')))
        reader.fieldnames = [name.strip() for name in reader.fieldnames]
        required = {'PROVINCE_NAME', 'AMPHUR_NAME', 'DISTRICT_CODE', *(f'Y{y}' for y in range(2562, 2568))}
        if not required.issubset(reader.fieldnames):
            raise ValueError('District-history schema changed')
        for row in reader:
            signature = tuple(str(row.get(k) or '').strip() for k in reader.fieldnames)
            if signature in seen:
                continue
            seen.add(signature)
            province, district = row['PROVINCE_NAME'].strip(), row['AMPHUR_NAME'].strip()
            if not province or not district:
                continue
            yearly = {y: count(row[f'Y{y}']) for y in range(2562, 2568)}
            if not any(yearly.values()):
                continue
            area = areas.setdefault((province, district), {'province': province, 'district': district,
                'reportedYears': set(), 'reportedVillageIncidents': 0, 'subdistricts': set()})
            area['reportedYears'].update(y for y, n in yearly.items() if n)
            area['reportedVillageIncidents'] += sum(yearly.values())
            subdistrict = row['DISTRICT_CODE'].strip() or row.get('DISTRICT_NAME', '').strip()
            if subdistrict:
                area['subdistricts'].add(subdistrict)
    districts = []
    for _, area in sorted(areas.items()):
        area['reportedYears'] = sorted(area['reportedYears'])
        area['affectedSubdistrictCount'] = len(area.pop('subdistricts'))
        districts.append(area)
    if len(districts) < 100:
        raise ValueError('Unexpectedly low district coverage')

    annual = package('dpm-gd027')
    resource = next(r for r in annual['resources'] if r['name'].startswith('2568') and r['format'].upper() == 'XLSX')
    payload = download(resource['url'])
    book = openpyxl.load_workbook(io.BytesIO(payload), read_only=True, data_only=True)
    reports = []
    fields = {'reportedOccurrences': 'Time', 'affectedDistrictCount': 'District',
              'affectedSubdistrictCount': 'Sub-district', 'affectedCommunityCount': 'Community',
              'affectedPeople': 'Affected People', 'affectedHouseholds': 'Affected Households'}
    for sheet in book:
        rows = iter(sheet.values)
        columns = [str(c or '').strip() for c in next(rows)]
        if not {'Province', 'Disaster Type', *fields.values()}.issubset(columns):
            continue
        for row in rows:
            record = dict(zip(columns, row))
            province = record['Province']
            if record['Disaster Type'] != 'อุทกภัย' or not isinstance(province, str) or 'ผลรวม' in str(record.get('region')):
                continue  # bilingual header, region totals and national total
            reports.append({'province': province.strip(), 'year': 2568,
                **{key: count(record[col]) for key, col in fields.items()},
                'geographicScope': 'province', 'license': annual['license_id'],
                'sourceUrl': resource['url']})
    book.close()
    if len(reports) != 74 or len({r['province'] for r in reports}) != 74:
        raise ValueError('Expected 74 reported province rows; review source changes')
    if any(r['province'] == 'จังหวัด' for r in reports):
        raise ValueError('Header incorrectly imported')
    return {'schemaVersion': 1, 'importedAt': datetime.now(timezone.utc).isoformat(),
        'license': history['license_id'], 'periodStartYear': 2562, 'periodEndYear': 2567,
        'districtSources': sources, 'provinceSource': provenance(resource, payload),
        'districts': districts, 'provinceReports': sorted(reports, key=lambda r: r['province'])}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=pathlib.Path, default=OUTPUT)
    args = parser.parse_args()
    snapshot = build_snapshot()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(snapshot, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print(f"Imported {len(snapshot['districts'])} districts (2562-2567), "
          f"{len(snapshot['provinceReports'])} provincial summaries (2568).")
