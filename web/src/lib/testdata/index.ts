// Test API responses captured from dmitriylewen/alpine:3.21.2
// Features: VEX referrer, attestation referrer, CVSS scores, vulnerability scan
//
// Regenerate by running the backend and executing:
//   IMG="dmitriylewen/alpine:3.21.2"
//   curl -s "http://localhost:8080/api/health" | python3 -m json.tool > health.json
//   curl -s "http://localhost:8080/api/inspect?image=$IMG" | python3 -m json.tool > inspect.json
//   curl -s "http://localhost:8080/api/matching-tags?image=$IMG" | python3 -m json.tool > matching-tags.json
//   curl -s "http://localhost:8080/api/vex?repository=dmitriylewen/alpine&digest=<vex-digest>" | python3 -m json.tool > vex.json
//   curl -s "http://localhost:8080/api/scan?image=$IMG" | python3 -m json.tool > scan.json

import healthRaw from './health.json';
import inspectRaw from './inspect.json';
import matchingTagsRaw from './matching-tags.json';
import vexRaw from './vex.json';
import scanRaw from './scan.json';

import type { HealthData, ImageInfo, MatchingTagsResult, VEXDocument, ScanResult } from '../types';

export const testHealth = (healthRaw as Record<string, unknown>).data as HealthData;
export const testInspect = (inspectRaw as Record<string, unknown>).data as ImageInfo;
export const testMatchingTags = (matchingTagsRaw as Record<string, unknown>).data as MatchingTagsResult;
export const testVex = (vexRaw as Record<string, unknown>).data as VEXDocument;
export const testScan = (scanRaw as Record<string, unknown>).data as ScanResult;
