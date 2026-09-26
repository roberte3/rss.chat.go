# Implementation Plan: RSS.Chat Go Enhancement

## Project Overview

This document outlines a plan to enhance the RSS.Chat Go implementation to match the feature set of the original JavaScript version and address known gaps.

## Current State Analysis

The Go implementation is already quite complete, implementing most core functionality:
- User management and authentication
- Post creation, updates, and deletion
- Threaded conversations
- Like/unlike functionality
- RSS feed generation
- Media upload with validation
- Real-time WebSocket updates
- Backup/restore tools

However, several key features from the original JavaScript implementation are missing or incomplete.

## Enhancement Priorities

### Phase 1: Critical Fixes (Immediate)
**Priority: High**

1. **Fix websocketPort Issue**
   - Issue: websocketPort (1462) is config-only - nothing listens on it
   - Solution: Make WebSocket endpoint available on the configured port or use main port
   - Status: Currently a known gap in implementation

2. **Implement `/version` Endpoint**
   - Issue: Called by client's debug helpers but not implemented
   - Solution: Create endpoint that returns server version
   - Status: Missing implementation

3. **Implement `/readhttpfile` Endpoint**
   - Issue: Not implemented, but needed for client Scripts menu
   - Solution: Add endpoint with proper SSRF protection
   - Status: Missing implementation

### Phase 2: Feature Implementation (Medium)
**Priority: Medium**

4. **Add RSSCloud Ping Support**
   - Issue: RSSCloud ping functionality missing
   - Solution: Implement RSSCloud ping to notify feed readers
   - Status: Missing feature

5. **Add Websub Hub Support**
   - Issue: Websub hub notifications missing
   - Solution: Implement WebSub hub support for feed updates
   - Status: Missing feature

6. **Database-backed Feed Storage**
   - Issue: Only filesystem storage supported
   - Solution: Add support for database-backed feed storage
   - Status: Partially implemented but not fully functional

### Phase 3: Advanced Features (Low)
**Priority: Low**

7. **Email Notifications**
   - Issue: No automated email notifications
   - Solution: Implement notification system for replies and likes
   - Status: Missing feature

8. **Post Cleanup/Archival**
   - Issue: No automatic post cleanup functionality
   - Solution: Add post cleanup/archival system
   - Status: Missing feature

9. **Advanced Blocking/Filtering**
   - Issue: Limited blocking functionality
   - Solution: Add sophisticated blocking/filtering
   - Status: Missing feature

## Detailed Implementation Plan

### Phase 1: Critical Fixes

#### 1. Fix websocketPort Issue
**Files to modify:**
- `main.go` - Update WebSocket server setup to listen on configured port
- `config/config.go` - Validate websocket port configuration

**Implementation steps:**
1. Modify WebSocket hub to accept port configuration
2. Update main.go to start WebSocket server on configured port
3. Update documentation about WebSocket port usage

#### 2. Implement `/version` Endpoint
**Files to modify:**
- `api/handler.go` - Add version endpoint
- `main.go` - Register version endpoint

**Implementation steps:**
1. Create version endpoint handler
2. Return server version information
3. Add test coverage

#### 3. Implement `/readhttpfile` Endpoint
**Files to modify:**
- `api/handler.go` - Add readhttpfile endpoint
- `api/endpoint_test.go` - Add tests

**Implementation steps:**
1. Create endpoint that accepts URL parameter
2. Implement SSRF protection (HTTPS only, address validation)
3. Add proper error handling
4. Add tests

### Phase 2: Feature Implementation

#### 4. Add RSSCloud Ping Support
**Files to modify:**
- `feed/feed.go` - Add RSSCloud ping functionality
- `api/handler.go` - Update feed publishing to include RSSCloud ping

**Implementation steps:**
1. Add RSSCloud ping function
2. Integrate with feed publishing logic
3. Add configuration options for RSSCloud
4. Add tests

#### 5. Add Websub Hub Support
**Files to modify:**
- `feed/feed.go` - Add Websub functionality
- `api/handler.go` - Update feed publishing to include Websub

**Implementation steps:**
1. Add Websub client functionality
2. Integrate with feed publishing logic
3. Add configuration options
4. Add tests

#### 6. Database-backed Feed Storage
**Files to modify:**
- `db/db.go` - Add database feed storage functions
- `publish/publisher.go` - Update publisher to support database mode
- `main.go` - Update configuration handling

**Implementation steps:**
1. Add database feed storage functions
2. Update publisher to handle database storage
3. Implement feed storage and retrieval from database
4. Add configuration options
5. Add tests

### Phase 3: Advanced Features

#### 7. Email Notifications
**Files to modify:**
- `email/email.go` - Add notification functionality
- `api/handler.go` - Add notification triggers
- `db/db.go` - Add notification tracking

**Implementation steps:**
1. Add email notification system
2. Implement notification triggers for replies/likes
3. Add configuration options
4. Add tests

#### 8. Post Cleanup/Archival
**Files to modify:**
- `db/db.go` - Add cleanup functions
- `api/handler.go` - Add cleanup endpoint
- `main.go` - Add cleanup scheduling

**Implementation steps:**
1. Add post cleanup logic
2. Implement archival system
3. Add configuration options
4. Add tests

#### 9. Advanced Blocking/Filtering
**Files to modify:**
- `db/db.go` - Add advanced blocking functions
- `api/handler.go` - Add blocking endpoints
- `config/config.go` - Add advanced blocking configuration

**Implementation steps:**
1. Add advanced blocking functionality
2. Implement filtering system
3. Add configuration options
4. Add tests

## Technical Considerations

### Security
- All new endpoints must include proper input validation
- `/readhttpfile` requires strict SSRF protection
- Email notification system must avoid spam issues
- Rate limiting should be applied to new endpoints

### Performance
- Database-backed feeds should be optimized for performance
- RSSCloud and Websub operations should be asynchronous
- Media handling should maintain performance

### Compatibility
- New features should not break existing functionality
- API endpoints should maintain backward compatibility
- Configuration should be backward compatible

## Testing Strategy

### Unit Tests
- Add tests for each new endpoint
- Add tests for database feed storage
- Add tests for RSSCloud and Websub functionality
- Add tests for email notifications

### Integration Tests
- Test complete workflow from post creation to feed generation
- Test WebSocket notifications
- Test media upload and retrieval
- Test database-backed feeds

### Security Tests
- Test SSRF protection for `/readhttpfile`
- Test rate limiting on new endpoints
- Test authentication on protected endpoints

## Timeline Estimate

### Phase 1: Critical Fixes (2-3 weeks)
- Fix websocketPort issue
- Implement `/version` endpoint
- Implement `/readhttpfile` endpoint

### Phase 2: Feature Implementation (4-6 weeks)
- Add RSSCloud ping support
- Add Websub hub support
- Implement database-backed feed storage

### Phase 3: Advanced Features (3-4 weeks)
- Email notifications
- Post cleanup/archival
- Advanced blocking/filtering

## Resource Requirements

### Development Resources
- 1-2 developers for 12-15 weeks
- Testing resources for comprehensive test coverage
- Documentation updates

### Testing Resources
- Automated test suite expansion
- Manual testing of new features
- Security testing for new endpoints

## Risk Assessment

### High Risk
- SSRF vulnerability in `/readhttpfile` endpoint
- Performance impact of database-backed feeds

### Medium Risk
- Security issues in email notification system
- Compatibility issues with existing features

### Low Risk
- Minor API changes
- Configuration complexity

## Success Metrics

1. All critical fixes implemented and tested
2. All new features working as specified
3. No regression in existing functionality
4. Performance maintained or improved
5. Security vulnerabilities addressed
6. Comprehensive test coverage (90%+)
7. Documentation updated appropriately

This plan provides a structured approach to enhance the RSS.Chat Go implementation to match the feature set of the original JavaScript version while maintaining quality and security standards.