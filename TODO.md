# Pomo Cloud Sync Implementation TODO

## Project Overview
Implement cloud synchronization for the pomo CLI tool, allowing users to sync their pomodoro sessions across multiple devices and access a web-based dashboard for statistics and session management.

## Architecture Overview

### Tech Stack
- **Backend**: Go with Gin/Fiber framework
- **Database**: SQLite (local) + PostgreSQL (server)
- **Authentication**: JWT tokens with refresh mechanism
- **Frontend**: Go templates + HTMX + Tailwind CSS
- **Hosting**: AWS (recommended) or Railway/Fly.io
- **ORM**: GORM for database operations

### Core Features
1. User authentication and account management
2. Session synchronization across devices
3. Conflict resolution for concurrent edits
4. Web dashboard for statistics and session management
5. CLI commands for sync operations
6. Hybrid database architecture (SQLite local + PostgreSQL server)
7. Self-updating CLI with version management
8. Rate limiting and automatic syncing
9. Background sync with intelligent batching

---

## 📋 Implementation Phases

### Phase 0: Hybrid Database Architecture & Sync System

#### 0.1 Hybrid Database Strategy
**Goal**: Keep SQLite locally (no installation required) + PostgreSQL on server

**Architecture Benefits:**
1. **No local installation** - Users don't need to install PostgreSQL
2. **Small binary size** - SQLite is embedded in Go
3. **Offline capability** - Works without internet connection
4. **Cloud sync** - Data synchronized to PostgreSQL server
5. **Multi-device support** - Access data from any device via server

#### 0.2 Sync System Implementation
**Files to create:**
- `cmd/sync.go` - Sync commands
- `internal/sync/sqlite_to_postgres.go` - Sync logic
- `internal/sync/validator.go` - Data validation
- `internal/cloud/client.go` - HTTP client for server communication

**Sync commands:**
```bash
pomo sync                     # Full sync (push + pull)
pomo push                     # Push local SQLite data to server PostgreSQL
pomo pull                     # Pull server PostgreSQL data to local SQLite
pomo sync --validate          # Validate sync integrity
pomo sync --status            # Show sync status
```

#### 0.3 Self-Update System
**Files to create:**
- `internal/version/version.go` - Version tracking
- `internal/update/updater.go` - Self-update logic
- `cmd/update.go` - Update command
- `internal/update/github.go` - GitHub releases integration

**Self-update commands:**
```bash
pomo version                    # Show current version
pomo update                     # Check for and install updates
pomo update --check            # Check for available updates
pomo update --force            # Force update even if same version
```

**Self-update features:**
- ✅ Automatic version checking via GitHub releases
- ✅ Secure binary verification with checksums
- ✅ Cross-platform support (Windows, macOS, Linux)
- ✅ Backup current binary before update
- ✅ Rollback capability if update fails
- ✅ Progress indicators during download

#### 0.4 Self-Update Workflow
**Complete update process:**
1. **Check version**: `pomo version` - Show current version
2. **Check for updates**: `pomo update --check` - Check for new version
3. **Install update**: `pomo update` - Download and install new version
4. **Register/login**: `pomo auth login your@email.com` - Set up cloud account
5. **Sync to cloud**: `pomo push` - Upload all local data to server
6. **Verify sync**: `pomo sync --status` - Ensure everything is synced

#### 0.5 Local Development Setup
**For developers only (users don't need this):**
```bash
# Using Docker for server development
docker run --name pomo-server-postgres \
  -e POSTGRES_DB=pomo_server \
  -e POSTGRES_USER=pomo \
  -e POSTGRES_PASSWORD=pomo123 \
  -p 5432:5432 \
  -d postgres:15
```

**Production setup (server only):**
- Use Railway PostgreSQL addon
- Use Fly.io PostgreSQL
- Use managed PostgreSQL service (AWS RDS, Google Cloud SQL)

**User experience:**
- ✅ No database installation required
- ✅ Just download and run the binary
- ✅ SQLite works out of the box
- ✅ Optional cloud sync when ready

#### 0.6 Database Schema Design
**Local SQLite schema (enhanced):**
```sql
-- Enhanced sessions table for sync
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT, -- UUID from server (NULL if not synced)
    type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
    topic TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_synced_at DATETIME,
    is_synced BOOLEAN DEFAULT FALSE
);

-- Sync metadata
CREATE TABLE sync_metadata (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Local configuration
CREATE TABLE local_config (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**Server PostgreSQL schema:**
```sql
-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Sessions table
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
    topic TEXT,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Device tracking
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) UNIQUE NOT NULL,
    device_name VARCHAR(255),
    last_seen TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);
```

---

### Phase 1: Server Foundation & Authentication

#### 1.1 Create Server Project Structure
```bash
mkdir pomo-server
cd pomo-server
go mod init github.com/Soeky/pomo-server
```

**Files to create:**
- `main.go` - Server entry point
- `internal/auth/` - Authentication handlers
- `internal/models/` - Database models
- `internal/handlers/` - HTTP handlers
- `internal/middleware/` - JWT middleware
- `internal/database/` - Database connection
- `config/` - Configuration management
- `web/` - Static files and templates

#### 1.2 Add Dependencies
```go
// go.mod additions for CLI (existing project)
require (
    gorm.io/gorm v1.25.5
    gorm.io/driver/postgres v1.5.4
    github.com/google/uuid v1.4.0
    github.com/joho/godotenv v1.4.0
)

// go.mod additions for server (new project)
require (
    github.com/gin-gonic/gin v1.9.1
    github.com/golang-jwt/jwt/v5 v5.0.0
    gorm.io/gorm v1.25.5
    gorm.io/driver/postgres v1.5.4
    github.com/google/uuid v1.4.0
    golang.org/x/crypto v0.15.0
    github.com/joho/godotenv v1.4.0
)
```

#### 1.3 Database Schema Design
```sql
-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Sessions table (extended from local)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    local_id INTEGER, -- Original local ID for mapping
    type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
    topic TEXT,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration INTEGER,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    last_synced_at TIMESTAMP DEFAULT NOW()
);

-- Sync metadata for conflict resolution
CREATE TABLE sync_metadata (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    last_sync_timestamp TIMESTAMP,
    device_id VARCHAR(255),
    device_name VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Device tracking
CREATE TABLE devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) UNIQUE NOT NULL,
    device_name VARCHAR(255),
    last_seen TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);
```

#### 1.4 Authentication System
**Files to implement:**
- `internal/auth/handlers.go` - Login/register endpoints
- `internal/auth/middleware.go` - JWT validation
- `internal/auth/utils.go` - Password hashing, token generation
- `internal/models/user.go` - User model

**Endpoints:**
```
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
```

#### 1.5 Configuration Management
**Files to create:**
- `config/config.go` - Configuration struct
- `.env.example` - Environment variables template
- `config/database.go` - Database configuration

---

### Phase 2: Session Management & Sync API

#### 2.1 Session Models and Handlers
**Files to implement:**
- `internal/models/session.go` - Session model with GORM
- `internal/handlers/sessions.go` - CRUD operations
- `internal/handlers/sync.go` - Sync operations

**API Endpoints:**
```
GET    /api/v1/sessions          - Get user sessions (with pagination)
POST   /api/v1/sessions          - Create new session
PUT    /api/v1/sessions/:id      - Update session
DELETE /api/v1/sessions/:id      - Delete session
GET    /api/v1/sessions/:id      - Get specific session
```

#### 2.2 Sync Operations
**Files to implement:**
- `internal/handlers/sync.go` - Sync logic
- `internal/sync/conflict_resolver.go` - Conflict resolution
- `internal/sync/merger.go` - Data merging logic

**Sync Endpoints:**
```
POST /api/v1/sync/push          - Push local changes to server
POST /api/v1/sync/pull          - Pull server changes to local
POST /api/v1/sync/conflicts     - Resolve sync conflicts
GET  /api/v1/sync/status        - Get sync status
```

#### 2.3 Conflict Resolution Strategy
**Implementation approach:**
1. **Timestamp-based**: Use `updated_at` timestamps
2. **Last-write-wins**: For simple conflicts
3. **Manual resolution**: For complex conflicts, provide UI
4. **Device priority**: Allow users to set device priority

**Conflict resolution flow:**
1. Compare local and server timestamps
2. If timestamps differ by < 5 minutes, auto-merge
3. If timestamps differ by > 5 minutes, flag for manual resolution
4. Provide conflict resolution API endpoint

---

### Phase 3: CLI Integration

#### 3.1 Add New Commands to Existing CLI
**Files to modify:**
- `cmd/root.go` - Add new command groups
- `internal/config/config.go` - Add cloud sync config

**New commands to add:**
```bash
# Authentication commands
pomo auth login <email>        # Login to cloud account
pomo auth logout              # Logout from cloud account
pomo auth register <email>    # Register new cloud account
pomo auth status              # Show authentication status

# Sync commands
pomo sync                     # Full sync (push + pull)
pomo push                     # Push local changes to server
pomo pull                     # Pull server changes to local
pomo sync status              # Show sync status and conflicts
pomo sync resolve             # Resolve sync conflicts interactively
pomo sync config              # Configure sync settings
pomo sync queue               # Show pending sync queue

# Auto-sync commands
pomo sync enable              # Enable automatic syncing
pomo sync disable             # Disable automatic syncing
pomo sync pause               # Pause syncing temporarily
pomo sync resume              # Resume syncing

# Enhanced status
pomo status --cloud           # Show status with cloud sync info
pomo status --sync            # Show detailed sync status
```

#### 3.2 Cloud Configuration
**Files to create:**
- `internal/cloud/config.go` - Cloud configuration
- `internal/cloud/client.go` - HTTP client for API calls
- `internal/cloud/auth.go` - Authentication management
- `internal/cloud/sync.go` - Sync operations

**Configuration structure:**
```go
type CloudConfig struct {
    ServerURL    string `json:"server_url"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    DeviceID     string `json:"device_id"`
    DeviceName   string `json:"device_name"`
    LastSync     time.Time `json:"last_sync"`
}
```

#### 3.3 Token Management
**Implementation:**
- Store tokens in secure keychain (macOS Keychain, Windows Credential Manager)
- Auto-refresh tokens when expired
- Handle authentication errors gracefully
- Provide clear error messages for auth failures

#### 3.4 Sync Logic Integration
**Files to modify:**
- `internal/db/postgres.go` - PostgreSQL connection and sync metadata tracking
- `internal/session/sessions.go` - Add cloud sync hooks
- `cmd/start.go` - Add sync after session creation
- `cmd/stop.go` - Add sync after session completion

**Sync triggers:**
- After each session start/stop
- On explicit sync command
- On app startup (if configured)
- Periodic background sync (optional)

#### 3.5 Rate Limiting & Automatic Sync
**Files to create:**
- `internal/sync/rate_limiter.go` - Client-side rate limiting
- `internal/sync/auto_sync.go` - Automatic background sync
- `internal/sync/batcher.go` - Intelligent batching of sync operations
- `internal/config/sync_config.go` - Sync configuration management

**Rate limiting features:**
- ✅ **Client-side rate limiting** - Prevent API spam
- ✅ **Exponential backoff** - Handle server errors gracefully
- ✅ **Request queuing** - Queue requests when rate limited
- ✅ **Configurable limits** - User can adjust sync frequency

**Automatic sync features:**
- ✅ **Background sync** - Sync automatically in background
- ✅ **Intelligent batching** - Batch multiple changes together
- ✅ **Smart triggers** - Sync on session start/stop, periodic intervals
- ✅ **Offline queue** - Queue changes when offline, sync when online
- ✅ **Conflict resolution** - Handle conflicts automatically

#### 3.6 Database Migration Integration
**Files to modify:**
- `internal/db/db.go` - Add PostgreSQL support alongside SQLite
- `internal/config/config.go` - Add database type configuration
- `cmd/root.go` - Add migration and upgrade commands

**Migration workflow integration:**
- Check for existing SQLite database on startup
- Prompt user to migrate if SQLite is found
- Provide seamless migration experience
- Maintain backward compatibility during transition

---

### Phase 4: Web Dashboard

#### 4.1 Web Server Setup
**Files to create:**
- `web/templates/` - HTML templates
- `web/static/` - CSS, JS, images
- `internal/handlers/web.go` - Web page handlers
- `internal/middleware/auth.go` - Web authentication

#### 4.2 Frontend Stack
**Technologies:**
- **Templates**: Go `html/template` or [Templ](https://templ.guide/)
- **Styling**: Tailwind CSS via CDN
- **Interactivity**: HTMX for dynamic updates
- **Charts**: Chart.js for statistics visualization
- **Icons**: Heroicons or Lucide icons

#### 4.3 Dashboard Pages
**Pages to implement:**
1. **Login/Register** (`/login`, `/register`)
2. **Dashboard** (`/`) - Overview with daily stats
3. **Sessions** (`/sessions`) - Session history with filtering
4. **Statistics** (`/stats`) - Detailed analytics
5. **Settings** (`/settings`) - Account and sync preferences
6. **Devices** (`/devices`) - Manage connected devices

#### 4.4 Real-time Features
**HTMX implementations:**
- Live session timer updates
- Real-time sync status
- Auto-refresh statistics
- Live notifications for conflicts

---

### Phase 5: Statistics & Analytics

#### 5.1 Statistics API
**Files to implement:**
- `internal/handlers/stats.go` - Statistics endpoints
- `internal/stats/calculator.go` - Statistics calculations
- `internal/stats/aggregator.go` - Data aggregation

**Statistics endpoints:**
```
GET /api/v1/stats/daily?date=2024-01-15
GET /api/v1/stats/weekly?week=2024-W03
GET /api/v1/stats/monthly?month=2024-01
GET /api/v1/stats/range?start=2024-01-01&end=2024-01-31
GET /api/v1/stats/topics
GET /api/v1/stats/streaks
```

#### 5.2 Analytics Features
**Metrics to track:**
- Daily/weekly/monthly focus time
- Break time distribution
- Topic-based productivity
- Session completion rates
- Productivity streaks
- Peak productivity hours
- Device usage patterns

#### 5.3 Data Visualization
**Chart types:**
- Line charts for time series data
- Bar charts for topic comparison
- Pie charts for session type distribution
- Heatmaps for productivity patterns
- Progress bars for goals

---

### Phase 6: AWS Deployment & Hosting

#### 6.1 AWS Hosting Options (Recommended)

**Option A: AWS App Runner (Easiest)**
- ✅ Fully managed service
- ✅ Automatic scaling
- ✅ Built-in load balancing
- ✅ Easy deployment from GitHub
- ✅ Automatic HTTPS
- ✅ Cost-effective for small to medium traffic

**Deployment steps:**
1. Create AWS App Runner service
2. Connect GitHub repository
3. Configure build settings (`apprunner.yaml`)
4. Set up RDS PostgreSQL database
5. Configure environment variables
6. Deploy automatically

**Option B: AWS Elastic Beanstalk (More Control)**
- ✅ Platform as a Service (PaaS)
- ✅ Easy deployment and management
- ✅ Built-in monitoring and logging
- ✅ Auto-scaling capabilities
- ✅ Load balancing included

**Deployment steps:**
1. Create Elastic Beanstalk application
2. Deploy Go application
3. Set up RDS PostgreSQL database
4. Configure environment variables
5. Set up custom domain

**Option C: AWS ECS with Fargate (Most Scalable)**
- ✅ Container-based deployment
- ✅ Serverless containers
- ✅ High scalability
- ✅ Full control over infrastructure
- ✅ Best for high-traffic applications

#### 6.2 AWS Infrastructure Setup

**Required AWS Services:**
- **App Runner/Elastic Beanstalk/ECS** - Application hosting
- **RDS PostgreSQL** - Database
- **Route 53** - DNS management
- **ACM (Certificate Manager)** - SSL certificates
- **CloudWatch** - Monitoring and logging
- **IAM** - Security and permissions

**Environment variables:**
```
DATABASE_URL=postgresql://username:password@rds-endpoint:5432/pomo
JWT_SECRET=your-secret-key
SERVER_PORT=8080
ENVIRONMENT=production
AWS_REGION=us-east-1
```

#### 6.3 Database Setup (RDS PostgreSQL)
**RDS Configuration:**
- **Instance class**: db.t3.micro (free tier) or db.t3.small
- **Storage**: 20GB minimum
- **Backup**: Automated backups enabled
- **Security**: VPC with security groups
- **Monitoring**: Enhanced monitoring enabled

#### 6.4 Domain & SSL Setup
- **Route 53**: Configure DNS records
- **ACM**: Request SSL certificate
- **Custom domain**: Configure in App Runner/Elastic Beanstalk
- **CORS**: Configure for CLI access

#### 6.5 Cost Estimation (Monthly)

**Free Tier Options (0-100 users):**
- **AWS Free Tier (12 months)**: $0/month
  - EC2 t2.micro: 750 hours/month (free)
  - RDS db.t2.micro: 750 hours/month (free)
  - S3: 5GB storage (free)
  - Route 53: $1/month (not free)
  - **Total: ~$1/month**

**Very Small Scale (1-100 users):**
- **Railway Free Tier**: $0/month
  - 500 hours of usage/month
  - 1GB RAM, 1GB storage
  - PostgreSQL included
  - **Total: $0/month**

- **AWS Lightsail**: $3.50/month
  - 512MB RAM, 1 vCPU
  - 20GB SSD storage
  - 1TB data transfer
  - **Total: $3.50/month**

**Small Scale (100-1000 users):**
- App Runner: $5-25/month
- RDS PostgreSQL (db.t3.micro): $15/month
- Route 53: $1/month
- **Total: ~$20-40/month**

**Medium Scale (1000-10000 users):**
- App Runner: $25-100/month
- RDS PostgreSQL (db.t3.small): $30/month
- Route 53: $1/month
- **Total: ~$55-130/month**

#### 6.6 Alternative Hosting Options

**Free Options:**
- **Railway Free Tier**: $0/month
  - 500 hours usage/month
  - 1GB RAM, 1GB storage
  - PostgreSQL included
  - Perfect for 1-100 users

- **Heroku Free Tier**: $0/month
  - 550-1000 dyno hours/month
  - Apps sleep after 30min inactivity
  - PostgreSQL addon available
  - Good for development/testing

- **Render Free Tier**: $0/month
  - 750 hours/month
  - 512MB RAM
  - PostgreSQL included
  - Automatic deployments

**Low-Cost Options:**
- **Railway Pro**: $5/month
  - Unlimited usage
  - 8GB RAM, 100GB storage
  - PostgreSQL included
  - Perfect for 100-1000 users

- **AWS Lightsail**: $3.50/month
  - 512MB RAM, 1 vCPU
  - 20GB SSD storage
  - 1TB data transfer
  - Need to set up PostgreSQL yourself

- **DigitalOcean App Platform**: $5/month
  - 512MB RAM
  - 1GB storage
  - Managed PostgreSQL available
  - Simple deployment

**Medium-Cost Options:**
- **Fly.io**: $10-50/month
  - Global edge deployment
  - Built-in PostgreSQL
  - Docker-based deployment
  - Great for global users

- **AWS App Runner**: $20-40/month
  - Fully managed
  - Automatic scaling
  - RDS PostgreSQL
  - Best for production

#### 6.7 Recommended Hosting by User Scale

**1-50 Users (Free Options):**
- **Railway Free Tier** - Best choice
  - $0/month
  - 500 hours usage/month
  - PostgreSQL included
  - Easy deployment from GitHub
  - Perfect for MVP and early users

**50-200 Users (Low Cost):**
- **Railway Pro** - Recommended
  - $5/month
  - Unlimited usage
  - 8GB RAM, 100GB storage
  - PostgreSQL included
  - Great value for money

**200-1000 Users (Medium Cost):**
- **AWS App Runner** - Recommended
  - $20-40/month
  - Fully managed
  - Automatic scaling
  - RDS PostgreSQL
  - Production-ready

**1000+ Users (High Scale):**
- **AWS ECS with Fargate** - Recommended
  - $50-200/month
  - Container-based
  - High scalability
  - Full control
  - Best for large applications

#### 6.8 Getting Started Recommendation

**For your first deployment (1-100 users):**
1. **Start with Railway Free Tier** - $0/month
2. **Deploy your Go server** - Connect GitHub repo
3. **Use built-in PostgreSQL** - No setup needed
4. **Upgrade to Railway Pro** - $5/month when you need more resources
5. **Migrate to AWS** - When you reach 500+ users

**Why Railway Free Tier is perfect for you:**
- ✅ **Completely free** for small usage
- ✅ **PostgreSQL included** - No database setup
- ✅ **Easy deployment** - Just connect GitHub
- ✅ **Automatic HTTPS** - SSL included
- ✅ **No credit card required** - Start immediately
- ✅ **Easy upgrade path** - Scale as you grow

---

### Phase 7: Testing & Quality Assurance

#### 7.1 Unit Tests
**Files to create:**
- `internal/auth/auth_test.go`
- `internal/handlers/sessions_test.go`
- `internal/sync/sync_test.go`
- `internal/stats/stats_test.go`

#### 7.2 Integration Tests
**Test scenarios:**
- Authentication flow
- Sync operations
- Conflict resolution
- API endpoints
- Database operations

#### 7.3 CLI Testing
**Test commands:**
- All new CLI commands
- Error handling
- Network failure scenarios
- Token expiration handling

---

### Phase 8: Documentation & User Experience

#### 8.1 Documentation
**Files to create:**
- `docs/CLOUD_SYNC.md` - Cloud sync documentation
- `docs/API.md` - API documentation
- `docs/DEPLOYMENT.md` - Deployment guide
- `docs/TROUBLESHOOTING.md` - Common issues

#### 8.2 User Experience
**Improvements:**
- Clear error messages
- Progress indicators for sync operations
- Offline mode support
- Graceful degradation when server is unavailable

#### 8.3 Migration Guide
**For existing users:**
- How to migrate existing data from SQLite to PostgreSQL
- Account setup instructions
- Sync configuration guide
- Version upgrade process

**Migration documentation:**
- `docs/MIGRATION.md` - Complete migration guide
- `docs/UPGRADE.md` - Version upgrade instructions
- `docs/TROUBLESHOOTING.md` - Common migration issues

---

## ⚡ Rate Limiting & Automatic Sync Implementation

### Rate Limiting Strategy

#### Client-Side Rate Limiting
**Implementation approach:**
- **Token bucket algorithm** - Allow bursts but limit average rate
- **Exponential backoff** - Increase delay on server errors
- **Request queuing** - Queue requests when rate limited
- **Configurable limits** - User can adjust sync frequency

**Rate limiting configuration:**
```go
type RateLimitConfig struct {
    RequestsPerMinute int           `json:"requests_per_minute"`
    BurstSize         int           `json:"burst_size"`
    BackoffMultiplier float64       `json:"backoff_multiplier"`
    MaxBackoffDelay   time.Duration `json:"max_backoff_delay"`
    QueueSize         int           `json:"queue_size"`
}
```

**Default rate limits:**
- **Normal sync**: 10 requests/minute
- **Burst sync**: 5 requests in 10 seconds
- **Background sync**: 5 requests/minute
- **Manual sync**: 20 requests/minute

#### Server-Side Rate Limiting
**Implementation:**
- **Per-user rate limiting** - Limit requests per user
- **Per-IP rate limiting** - Prevent abuse
- **Endpoint-specific limits** - Different limits for different endpoints
- **Graceful degradation** - Return 429 with retry-after header

### Automatic Sync System

#### Sync Triggers
**Automatic sync triggers:**
1. **Session events** - Start/stop session
2. **Periodic sync** - Every 5-15 minutes
3. **App startup** - Sync on application start
4. **Network reconnection** - Sync when coming back online
5. **Idle detection** - Sync when user is idle

#### Intelligent Batching
**Batching strategy:**
- **Time-based batching** - Batch changes within 30 seconds
- **Size-based batching** - Batch up to 50 changes
- **Priority batching** - Prioritize recent changes
- **Conflict-aware batching** - Handle conflicts in batches

#### Offline Support
**Offline queue:**
- **Local queue** - Store changes when offline
- **Conflict detection** - Detect conflicts when coming online
- **Automatic retry** - Retry failed syncs
- **Data integrity** - Ensure no data loss

### Implementation Files

#### `internal/sync/rate_limiter.go`
```go
package sync

import (
    "context"
    "time"
)

type RateLimiter struct {
    config     RateLimitConfig
    tokens     int
    lastUpdate time.Time
    queue      chan SyncRequest
}

func (rl *RateLimiter) Allow() bool {
    // Token bucket implementation
    // Return true if request is allowed
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
    // Wait for rate limit to allow request
    // Implement exponential backoff
}
```

#### `internal/sync/auto_sync.go`
```go
package sync

import (
    "context"
    "time"
)

type AutoSync struct {
    config    SyncConfig
    rateLimit *RateLimiter
    batcher   *Batcher
    queue     *OfflineQueue
}

func (as *AutoSync) Start(ctx context.Context) {
    // Start background sync goroutine
    // Handle sync triggers
    // Manage offline queue
}

func (as *AutoSync) TriggerSync(reason SyncReason) {
    // Trigger sync based on reason
    // Batch changes intelligently
    // Handle rate limiting
}
```

#### `internal/sync/batcher.go`
```go
package sync

import (
    "time"
)

type Batcher struct {
    config     BatchConfig
    changes    []Change
    lastBatch  time.Time
    batchTimer *time.Timer
}

func (b *Batcher) AddChange(change Change) {
    // Add change to batch
    // Trigger batch if conditions met
}

func (b *Batcher) Flush() error {
    // Send all batched changes
    // Handle conflicts
    // Update sync status
}
```

### Sync Configuration

#### User-Configurable Settings
```go
type SyncConfig struct {
    AutoSyncEnabled    bool          `json:"auto_sync_enabled"`
    SyncInterval       time.Duration `json:"sync_interval"`
    BackgroundSync     bool          `json:"background_sync"`
    OfflineQueue       bool          `json:"offline_queue"`
    ConflictResolution string        `json:"conflict_resolution"`
    RateLimit          RateLimitConfig `json:"rate_limit"`
}
```

#### Default Configuration
```json
{
    "auto_sync_enabled": true,
    "sync_interval": "5m",
    "background_sync": true,
    "offline_queue": true,
    "conflict_resolution": "timestamp",
    "rate_limit": {
        "requests_per_minute": 10,
        "burst_size": 5,
        "backoff_multiplier": 2.0,
        "max_backoff_delay": "60s",
        "queue_size": 100
    }
}
```

### User Experience

#### Sync Status Indicators
- **🟢 Synced** - All changes synced
- **🟡 Syncing** - Currently syncing
- **🔴 Offline** - No network connection
- **⚠️ Conflicts** - Conflicts need resolution
- **📊 Queued** - Changes waiting to sync

#### Progress Indicators
- **Sync progress** - Show sync progress
- **Queue size** - Show pending changes
- **Last sync** - Show last sync time
- **Next sync** - Show next scheduled sync

#### Error Handling
- **Network errors** - Queue changes for later
- **Server errors** - Exponential backoff
- **Authentication errors** - Prompt for re-login
- **Conflict errors** - Show conflict resolution UI

---

## 🔄 Self-Update Implementation Details

### Self-Update Architecture
**How the self-update system works:**

1. **Version Checking**: Query GitHub releases API for latest version
2. **Binary Download**: Download new binary from GitHub releases
3. **Verification**: Verify binary integrity with checksums
4. **Backup**: Create backup of current binary
5. **Replacement**: Replace current binary with new version
6. **Restart**: Restart application with new version

### Implementation Files

#### `internal/version/version.go`
```go
package version

import (
    "fmt"
    "runtime"
)

const Version = "1.0.0"

type VersionInfo struct {
    Version   string `json:"version"`
    BuildTime string `json:"build_time"`
    GoVersion string `json:"go_version"`
    Platform  string `json:"platform"`
}

func GetVersionInfo() VersionInfo {
    return VersionInfo{
        Version:   Version,
        BuildTime: "2024-01-01T00:00:00Z",
        GoVersion: runtime.Version(),
        Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
    }
}
```

#### `internal/update/updater.go`
```go
package update

import (
    "crypto/sha256"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "runtime"
)

type Updater struct {
    CurrentVersion string
    GitHubRepo     string
    BinaryName     string
}

func NewUpdater(repo string) *Updater {
    return &Updater{
        CurrentVersion: version.Version,
        GitHubRepo:     repo,
        BinaryName:     getBinaryName(),
    }
}

func (u *Updater) CheckForUpdates() (*Release, error) {
    // Query GitHub releases API
    // Compare with current version
    // Return latest release if newer
}

func (u *Updater) DownloadAndUpdate(release *Release) error {
    // Download new binary
    // Verify checksum
    // Backup current binary
    // Replace current binary
    // Restart application
}

func getBinaryName() string {
    name := "pomo"
    if runtime.GOOS == "windows" {
        name += ".exe"
    }
    return name
}
```

#### `cmd/update.go`
```go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/Soeky/pomo/internal/update"
)

var updateCmd = &cobra.Command{
    Use:   "update",
    Short: "Update pomo to the latest version",
    Long:  `Check for and install the latest version of pomo from GitHub releases.`,
    Run: func(cmd *cobra.Command, args []string) {
        updater := update.NewUpdater("Soeky/pomo")
        
        if checkOnly, _ := cmd.Flags().GetBool("check"); checkOnly {
            checkForUpdates(updater)
            return
        }
        
        if force, _ := cmd.Flags().GetBool("force"); force {
            forceUpdate(updater)
            return
        }
        
        performUpdate(updater)
    },
}

func init() {
    rootCmd.AddCommand(updateCmd)
    updateCmd.Flags().Bool("check", false, "Only check for updates, don't install")
    updateCmd.Flags().Bool("force", false, "Force update even if same version")
}
```

### GitHub Releases Setup
**Required for self-update to work:**

1. **Create GitHub releases** with version tags (e.g., v1.0.0, v1.0.1)
2. **Upload binaries** for each platform:
   - `pomo-darwin-amd64` (macOS Intel)
   - `pomo-darwin-arm64` (macOS Apple Silicon)
   - `pomo-linux-amd64` (Linux)
   - `pomo-windows-amd64.exe` (Windows)
3. **Include checksums** for security verification
4. **Use semantic versioning** (semver) for version comparison

### Security Considerations
- **Checksum verification**: Verify downloaded binary integrity
- **HTTPS only**: Download only over secure connections
- **GitHub releases**: Use official GitHub releases API
- **Backup mechanism**: Always backup current binary before update
- **Rollback capability**: Allow users to rollback if update fails

### User Experience
- **Progress indicators**: Show download progress
- **Clear messaging**: Inform users about update process
- **Confirmation prompts**: Ask before updating
- **Error handling**: Graceful handling of update failures
- **Automatic restart**: Seamless transition to new version

---

## 🔄 Migration & Upgrade Workflow

### User Upgrade Process
**For existing users upgrading to cloud sync version:**

1. **Check current version:**
   ```bash
   pomo version
   ```

2. **Download and install new version:**
   ```bash
   pomo upgrade
   ```

3. **Register/login to cloud account:**
   ```bash
   pomo auth register your@email.com
   # or
   pomo auth login your@email.com
   ```

4. **Push all local data to cloud:**
   ```bash
   pomo push
   ```

5. **Verify sync status:**
   ```bash
   pomo sync status
   ```

**That's it! No database migration needed.**

### Sync Safety Features
- **Offline capability**: Works without internet connection
- **Data validation**: Comprehensive validation of synced data
- **Conflict resolution**: Handle conflicts when syncing between devices
- **Progress tracking**: Clear progress indicators during sync
- **Error handling**: Graceful handling of sync failures
- **Backup creation**: Automatic backup before major sync operations

### Version Management
- **Semantic versioning**: Follow semver (e.g., v1.2.3)
- **Update notifications**: Check for updates on startup
- **Automatic updates**: Optional automatic update installation
- **Migration tracking**: Track which migrations have been applied

---

## 🔧 Development Workflow

### Local Development Setup
1. **Database setup (first time):**
   ```bash
   # Using Docker for local PostgreSQL
   docker run --name pomo-postgres \
     -e POSTGRES_DB=pomo \
     -e POSTGRES_USER=pomo \
     -e POSTGRES_PASSWORD=pomo123 \
     -p 5432:5432 \
     -d postgres:15
   ```

2. **CLI development:**
   ```bash
   cd /path/to/pomo
   # Migrate from SQLite to PostgreSQL
   go run main.go migrate --from-sqlite --to-postgres
   # Test new commands
   go run main.go sync --help
   ```

3. **Server development:**
   ```bash
   cd pomo-server
   go run main.go
   ```

4. **Migration testing:**
   ```bash
   # Test migration process
   go run main.go migrate --validate
   # Test rollback if needed
   go run main.go migrate --rollback
   ```

### Testing Strategy
1. **Unit tests** for all new functions
2. **Integration tests** for API endpoints
3. **End-to-end tests** for CLI commands
4. **Load testing** for sync operations

### Code Review Checklist
- [ ] Authentication security
- [ ] Input validation
- [ ] Error handling
- [ ] Database transactions
- [ ] API documentation
- [ ] CLI help text
- [ ] Test coverage

---

## 📊 Success Metrics

### Technical Metrics
- API response time < 200ms
- Sync operation completion < 5 seconds
- 99.9% uptime
- Zero data loss during sync

### User Experience Metrics
- Successful sync rate > 95%
- User onboarding completion > 80%
- Daily active users
- Session completion rate

---

## 🚀 Launch Plan

### Beta Release
1. Deploy server to staging environment
2. Test with limited user group
3. Gather feedback and iterate
4. Fix critical bugs

### Production Release
1. Deploy to production
2. Announce feature to community
3. Provide migration documentation
4. Monitor usage and performance

### Post-Launch
1. Monitor error rates and performance
2. Gather user feedback
3. Plan additional features
4. Optimize based on usage patterns

---

## 🔮 Future Enhancements

### Advanced Features
- **Team collaboration** - Share sessions with team members
- **Goal setting** - Set and track productivity goals
- **Integrations** - Connect with calendar apps, task managers
- **Mobile app** - Native mobile application
- **Offline mode** - Full offline functionality with sync when online

### Analytics Enhancements
- **AI insights** - Productivity pattern analysis
- **Predictive analytics** - Optimal work time suggestions
- **Export features** - Data export to CSV, PDF
- **Custom reports** - User-defined analytics

---

## 📝 Notes

- **Security**: Always use HTTPS in production
- **Privacy**: Consider data encryption for sensitive information
- **Scalability**: Design for horizontal scaling from the start
- **Monitoring**: Implement comprehensive logging and monitoring
- **Backup**: Regular database backups and disaster recovery plan

---

*This TODO document should be updated as the project progresses. Each completed item should be marked with ✅ and moved to a "Completed" section.*
