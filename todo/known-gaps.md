# Known Gaps in RSS.Chat Go Implementation

## Overview

This document details the known gaps and missing features in the Go implementation of RSS.Chat compared to the original JavaScript version.

## Critical Issues

### 1. WebSocket Port Not Listening
**Issue**: The `websocketPort` (1462) is configured in the config but nothing actually listens on it.

**Impact**: 
- In-browser real-time updates connect to a dead address
- WebSocket functionality is partially broken
- Client tries to connect to ws://localhost:1462/ but fails

**Status**: Known gap, requires fix to make WebSocket server listen on configured port

### 2. Missing `/version` Endpoint
**Issue**: The `/version` endpoint is called by the client's debug helpers but is not implemented.

**Impact**: 
- Client debug functionality shows incomplete version info
- No way to programmatically check server version
- May cause client-side errors in debug mode

**Status**: Missing implementation

### 3. Missing `/readhttpfile` Endpoint
**Issue**: The `/readhttpfile` endpoint is not implemented, which prevents the client's Scripts menu from working.

**Impact**:
- Client Scripts menu never appears
- No way to fetch external OPML files through the server
- Security risk - this was intended to be an authenticated SSRF endpoint

**Status**: Missing implementation

## Feature Gaps

### 4. RSSCloud Ping Support
**Issue**: The original JavaScript implementation had RSSCloud ping functionality for feed notifications.

**Impact**: 
- Feed readers may not get notified of updates
- Reduced discoverability of new content
- Missing integration with RSSCloud ecosystem

**Status**: Missing feature

### 5. Websub Hub Support
**Issue**: The original JavaScript implementation had Websub hub support.

**Impact**:
- Feed readers cannot subscribe to updates via WebSub
- Reduced real-time feed update capabilities
- Missing modern feed notification protocol

**Status**: Missing feature

### 6. Database-backed Feed Storage
**Issue**: While database storage is supported in code, it's not fully functional or tested.

**Impact**:
- Only filesystem storage works reliably
- Limited scalability for large deployments
- Missing integration with database storage mode

**Status**: Partially implemented, needs full implementation and testing

## Missing Functionality

### 7. Email Notifications
**Issue**: The original JavaScript implementation had email notification system.

**Impact**:
- Users don't get email notifications for replies/likes
- Reduced user engagement features
- Missing social interaction features

**Status**: Missing feature

### 8. Post Cleanup/Archival
**Issue**: The original JavaScript had plans for post cleanup and archival functionality.

**Impact**:
- Posts accumulate indefinitely
- Database growth issues
- No automated cleanup of old content

**Status**: Missing feature

### 9. Advanced Blocking/Filtering
**Issue**: The original had more sophisticated blocking and filtering mechanisms.

**Impact**:
- Limited moderation capabilities
- No advanced user blocking features
- Reduced content control

**Status**: Missing feature

### 10. Feed Format Negotiation
**Issue**: The original supported different feed formats.

**Impact**:
- Limited flexibility in feed consumption
- No support for JSON feeds or other formats
- Reduced compatibility with feed readers

**Status**: Missing feature

## Implementation Issues

### 11. `/localnewuser` Behavior Difference
**Issue**: The `/localnewuser` endpoint returns JSON instead of redirect like the original.

**Impact**:
- Different behavior from original
- May break tooling that depends on redirect behavior
- Different API contract

**Status**: Known behavioral difference

### 12. Media Upload Validation
**Issue**: While media validation exists, it may not match the original JavaScript implementation exactly.

**Impact**:
- Possible differences in edge case handling
- Less comprehensive validation than original
- Potential security differences

**Status**: Partial implementation, may need refinement

## Technical Debt

### 13. Configuration Inconsistencies
**Issue**: Some configuration options aren't fully utilized or implemented.

**Impact**:
- Configuration options are partially implemented
- Some flags in config don't have effect
- Inconsistent behavior with configuration

**Status**: Inconsistent implementation

### 14. Documentation Gaps
**Issue**: Some features are missing documentation or have incomplete documentation.

**Impact**:
- Harder to use advanced features
- Documentation doesn't match implementation
- Users may not know about available features

**Status**: Documentation gaps exist

## Security Considerations

### 15. SSRF Protection for `/readhttpfile`
**Issue**: The missing `/readhttpfile` endpoint represents a security gap.

**Impact**:
- Potential for SSRF attacks if implemented incorrectly
- No authentication mechanism for external file access
- Risk of accessing internal network resources

**Status**: Missing security implementation

## Testing Gaps

### 16. Test Coverage for New Features
**Issue**: Tests for missing features are not implemented.

**Impact**:
- No automated testing for missing features
- Harder to verify future implementations
- Risk of regressions

**Status**: Missing test coverage

## Recommendations

### Immediate Actions
1. Fix websocketPort to actually listen on configured port
2. Implement `/version` endpoint
3. Implement `/readhttpfile` endpoint with proper SSRF protection

### Short-term Goals (1-2 months)
1. Add RSSCloud ping support
2. Add Websub hub support
3. Implement database-backed feed storage properly
4. Implement basic email notification system

### Long-term Goals (3-6 months)
1. Implement full post cleanup/archival system
2. Add advanced blocking/filtering features
3. Add feed format negotiation
4. Complete comprehensive test coverage

## Priority Matrix

| Priority | Issue | Description |
|----------|-------|-------------|
| Critical | WebSocket Port | Nothing listens on configured port |
| High | Missing Endpoints | `/version` and `/readhttpfile` |
| Medium | RSS Features | RSSCloud, Websub support |
| Medium | Advanced Features | Email notifications, cleanup |
| Low | Implementation Details | `/localnewuser` behavior, documentation |

This document serves as a living reference for tracking the current gaps in the implementation and guiding future development efforts.