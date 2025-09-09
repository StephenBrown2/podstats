CREATE TABLE android_metadata (locale TEXT);
CREATE TABLE teams (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    home_page TEXT NOT NULL,
    version INTEGER NOT NULL,
    banner_id INTEGER NOT NULL,
    thumbnail_id INTEGER,
    store_url TEXT,
    language TEXT NOT NULL DEFAULT 'fr',
    last_modification_timestamp INTEGER NOT NULL DEFAULT -1,
    priority INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE sqlite_sequence(name, seq);
CREATE TABLE bitmaps (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT UNIQUE NOT NULL,
    is_asset INTEGER NOT NULL,
    is_downloaded INTEGER NOT NULL,
    local_file TEXT,
    md5 TEXT
);
CREATE TABLE podcasts (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    team_id INTEGER NOT NULL,
    category TEXT,
    type TEXT NOT NULL,
    subscribed_status INTEGER NOT NULL,
    version INTEGER NOT NULL,
    homepage TEXT,
    latest_publication_date INTEGER NOT NULL,
    feed_url TEXT UNIQUE NOT NULL collate nocase,
    favorite INTEGER,
    rating REAL NOT NULL,
    update_status INTEGER NOT NULL,
    update_date INTEGER,
    thumbnail_id INTEGER,
    store_url TEXT,
    last_modified INTEGER NOT NULL,
    etag TEXT NOT NULL,
    initialized_status INTEGER NOT NULL DEFAULT 0,
    CHARSET TEXT DEFAULT '',
    last_update_failure INTEGER NOT NULL DEFAULT 0,
    flattr TEXT DEFAULT '',
    is_virtual INTEGER DEFAULT 0,
    is_complete INTEGER DEFAULT 1,
    language TEXT,
    author TEXT,
    description TEXT,
    custom_name TEXT,
    priority INTEGER NOT NULL DEFAULT 1,
    accept_audio INTEGER NOT NULL DEFAULT 1,
    accept_video INTEGER NOT NULL DEFAULT 1,
    accept_text INTEGER NOT NULL DEFAULT 1,
    filter_included_keywords TEXT,
    filter_excluded_keywords TEXT,
    update_error_message TEXT,
    authentication INTEGER DEFAULT 0,
    login TEXT DEFAULT '',
    password TEXT DEFAULT '',
    automaticRefresh INTEGER DEFAULT 1,
    folderName TEXT DEFAULT '',
    private INTEGER(1) DEFAULT 0,
    iTunesID TEXT DEFAULT '',
    subscribers INTEGER DEFAULT -1,
    averageDuration INTEGER DEFAULT -1,
    frequency INTEGER DEFAULT -1,
    episodesNb INTEGER DEFAULT -1,
    explicit INTEGER(1) DEFAULT 0,
    iTunesType INTEGER DEFAULT 0,
    reviews INTEGER DEFAULT 0,
    position INTEGER DEFAULT 0,
    server_id INTEGER DEFAULT -1,
    muted INTEGER(1) DEFAULT 0,
    hub_url TEXT,
    topic_url TEXT,
    websubSubscribed INTEGER(1) DEFAULT 0,
    filter_chapter_excluded_keywords TEXT DEFAULT NULL,
    last_played_episode_date INTEGER DEFAULT -1,
    guid TEXT DEFAULT NULL collate nocase,
    liveStreamId INTEGER NOT NULL DEFAULT -1,
    liveStreamStart INTEGER NOT NULL DEFAULT -1,
    liveStreamEnd INTEGER NOT NULL DEFAULT -1,
    liveStreamGuid TEXT DEFAULT NULL collate nocase,
    liveStreamStatus INTEGER(1) NOT NULL DEFAULT 2,
    next_episode_forecast_date INTEGER NOT NULL DEFAULT -1,
    UNIQUE(feed_url) ON CONFLICT REPLACE
);
CREATE TABLE episodes (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL collate nocase,
    podcast_id INTEGER NOT NULL,
    guid TEXT NOT NULL,
    url TEXT,
    comments TEXT,
    publication_date INTEGER NOT NULL,
    creator TEXT,
    categories TEXT,
    description TEXT collate nocase,
    short_description TEXT collate nocase,
    content TEXT collate nocase,
    thumbnail_url TEXT,
    comment_rss TEXT,
    download_url TEXT,
    type TEXT,
    duration TEXT,
    size INTEGER NOT NULL,
    rating REAL NOT NULL DEFAULT -1,
    downloaded_status TEXT NOT NULL DEFAULT '-1',
    favorite INTEGER NOT NULL,
    seen_status INTEGER NOT NULL,
    downloaded_date INTEGER,
    new_status INTEGER NOT NULL,
    playing_status INTEGER NOT NULL DEFAULT 0,
    position_to_resume INTEGER,
    deleted_status INTEGER NOT NULL,
    local_file_name TEXT,
    thumbnail_id INTEGER,
    comments_last_modified INTEGER NOT NULL,
    comments_etag TEXT NOT NULL,
    duration_ms INTEGER NOT NULL DEFAULT -1,
    flattr TEXT DEFAULT '',
    is_virtual INTEGER DEFAULT 0,
    is_artwork_extracted INTEGER DEFAULT 0,
    virtualPodcastName TEXT DEFAULT '',
    normalizedType INTEGER DEFAULT 0,
    hasBeenFlattred INTEGER(1) DEFAULT 0,
    playbackDate INTEGER NOT NULL DEFAULT -1,
    download_error_msg TEXT,
    media_extracted_artwork_id INTEGER DEFAULT -1,
    chapters_extracted INTEGER(1) DEFAULT 0,
    server_id INTEGER DEFAULT -1,
    automatically_shared INTEGER(1) DEFAULT 0,
    downloaded_status_int INTEGER(1) NOT NULL DEFAULT 0,
    donation_url TEXT,
    explicit INTEGER(1) DEFAULT 0,
    iTunesType INTEGER DEFAULT 0,
    seasonNb INTEGER DEFAULT -1,
    episodeNb INTEGER DEFAULT -1,
    transcript_url TEXT,
    chapters_url TEXT,
    seasonName TEXT,
    thumbsRating INTEGER(1) NOT NULL DEFAULT 0,
    alternate_urls TEXT DEFAULT NULL,
    rssfeed_duration_ms INTEGER NOT NULL DEFAULT -1,
    chapter_origin INTEGER DEFAULT 0
);
CREATE TABLE comments (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    episode_id INTEGER NOT NULL,
    podcast_id INTEGER NOT NULL,
    title TEXT,
    creator TEXT NOT NULL,
    link TEXT NOT NULL,
    pubdate INTEGER NOT NULL,
    guid TEXT NOT NULL,
    description TEXT,
    content TEXT,
    new_status INTEGER NOT NULL
);
CREATE TABLE supported_languages (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    long_name TEXT UNIQUE NOT NULL,
    short_name TEXT UNIQUE NOT NULL
);
CREATE TABLE ordered_list (
    rank INTEGER NOT NULL,
    id INTEGER NOT NULL,
    type INTEGER NOT NULL,
    filter INTEGER DEFAULT -1
);
CREATE TABLE timestamp_list (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    type INTEGER NOT NULL
);
CREATE TABLE server_action (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    type INTEGER NOT NULL,
    entity INTEGER,
    entityId INTEGER NOT NULL DEFAULT -1,
    value INTEGER,
    timestamp INTEGER NOT NULL
);
CREATE TABLE tags (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    refresh_frequency INTEGER NOT NULL DEFAULT -1,
    refresh_time INTEGER NOT NULL DEFAULT -1,
    UNIQUE(name) ON CONFLICT REPLACE
);
CREATE TABLE tag_relation (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id INTEGER NOT NULL DEFAULT -1,
    podcast_id INTEGER NOT NULL DEFAULT -1,
    UNIQUE(tag_id, podcast_id) ON CONFLICT REPLACE
);
CREATE TABLE genres (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    UNIQUE(name) ON CONFLICT REPLACE
);
CREATE TABLE genre_relation (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    genre_id INTEGER NOT NULL DEFAULT -1,
    radio_id INTEGER NOT NULL DEFAULT -1,
    UNIQUE(genre_id, radio_id) ON CONFLICT REPLACE
);
CREATE TABLE statistics (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    entityType INTEGER NOT NULL,
    entityId INTEGER NOT NULL,
    entityStringId TEXT,
    type INTEGER NOT NULL,
    value INTEGER DEFAULT 0,
    timestamp INTEGER NOT NULL
);
CREATE TABLE radio_search_results (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT NOT NULL collate nocase,
    name TEXT NOT NULL collate nocase,
    country TEXT,
    language TEXT,
    genre TEXT,
    description TEXT,
    thumbnail_id INTEGER,
    quality INTEGER,
    tuneinID TEXT DEFAULT '',
    serverId INTEGER,
    episodeId INTEGER,
    countryCode TEXT,
    subscribers INTEGER,
    type INTEGER NOT NULL,
    UNIQUE (url, type)
);
CREATE TABLE chapters (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    podcastId INTEGER NOT NULL,
    episodeId INTEGER NOT NULL,
    start INTEGER NOT NULL,
    name TEXT,
    description TEXT,
    url TEXT,
    artworkId INTEGER,
    customBookmark INTEGER(1) DEFAULT 0,
    diaporamaFlag INTEGER(1) DEFAULT 0,
    updateDate INTEGER DEFAULT 0,
    isMuted INTEGER DEFAULT 0,
    isMusic INTEGER(1) DEFAULT 0,
    loopMode INTEGER(1) DEFAULT 0
);
CREATE TABLE popular_search_terms (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    languages TEXT NOT NULL,
    keywords TEXT NOT NULL
);
CREATE TABLE content_policy_violation (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT
);
CREATE TABLE relatedPodcasts (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    relation INTEGER NOT NULL,
    url TEXT NOT NULL,
    similar_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    score INTEGER NOT NULL default -1
);
CREATE TABLE reviews (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    serverId INTEGER NOT NULL,
    podcast_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    date INTEGER NOT NULL,
    isMyReview INTEGER DEFAULT 0,
    hasBeenFlagged INTEGER DEFAULT 0,
    rating INTEGER NOT NULL,
    comment TEXT,
    UNIQUE(serverId, podcast_id) ON CONFLICT REPLACE
);
CREATE TABLE ad_campaign (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT,
    podcast_id INTEGER NOT NULL DEFAULT 0,
    server_id INTEGER NOT NULL,
    language TEXT NOT NULL,
    enabled INTEGER(1) NOT NULL,
    paid_advertisement INTEGER(1) NOT NULL,
    category_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    artwork_id INTEGER NOT NULL DEFAULT -1,
    feature_in_popular_search_terms INTEGER(1) NOT NULL DEFAULT 0,
    search_terms TEXT DEFAULT NULL
);
CREATE TABLE curated_lists (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    type INTEGER NOT NULL DEFAULT 0,
    language TEXT NOT NULL,
    enabled INTEGER(1) NOT NULL,
    position INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    banner_id TEXT NOT NULL,
    header_id TEXT DEFAULT NULL
);
CREATE TABLE blocking_services (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    type INTEGER NOT NULL,
    pattern TEXT NOT NULL
);
CREATE TABLE iha (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    format INTEGER(1) NOT NULL,
    url TEXT NOT NULL,
    enabled INTEGER(1) NOT NULL,
    artwork_portrait_id INTEGER NOT NULL DEFAULT -1,
    artwork_landscape_id INTEGER NOT NULL DEFAULT -1
);
CREATE INDEX epPodcastIdSeenStatus_idx ON episodes(podcast_id, seen_status);
CREATE INDEX epSeenDlStatus_idx ON episodes(downloaded_status_int, seen_status);
CREATE INDEX epSeenFavoriteStatus_idx ON episodes(favorite, seen_status);
CREATE INDEX epNewStatusPodId_idx ON episodes(new_status, podcast_id);
CREATE INDEX epPodcastId_idx ON episodes(podcast_id);
CREATE INDEX epDownloadedStatus_idx ON episodes(downloaded_status_int);
CREATE INDEX epSeen_idx ON episodes(seen_status);
CREATE INDEX episodesFavorite_idx ON episodes(favorite);
CREATE INDEX episodesDownloadedDate_idx ON episodes(downloaded_date);
CREATE INDEX episodesSize_idx ON episodes(size);
CREATE INDEX episodesName_idx ON episodes(name COLLATE NOCASE);
CREATE INDEX episodesRating_idx ON episodes(rating);
CREATE INDEX episodesDurationMs_idx ON episodes(duration_ms);
CREATE INDEX episodesMimeType_idx ON episodes(type);
CREATE INDEX episodesIsVirtual_idx ON episodes(is_virtual);
CREATE INDEX episodesThumbnailId_idx ON episodes(thumbnail_id);
CREATE INDEX episodesLocalFileName_idx ON episodes(local_file_name);
CREATE INDEX episodesPlaybackDate_idx ON episodes(playbackDate);
CREATE INDEX episodesPositionToResume_idx ON episodes(podcast_id, position_to_resume);
CREATE INDEX episodesNormalizedTypeDL_idx ON episodes(normalizedType, downloaded_status_int);
CREATE INDEX episodesNormalizedType_idx ON episodes(normalizedType);
CREATE INDEX episodesNormalizedTypeComp_idx ON episodes(
    podcast_id,
    normalizedType,
    downloaded_status_int,
    seen_status
);
CREATE INDEX podName_idx ON podcasts(name COLLATE NOCASE);
CREATE INDEX podUpdateStatus_idx ON podcasts(update_status);
CREATE INDEX podSubscriptionStatus_idx ON podcasts(subscribed_status);
CREATE INDEX podSubsPriority_idx ON podcasts(subscribed_status, priority);
CREATE INDEX podTeamId_idx ON podcasts(team_id);
CREATE INDEX podType_idx ON podcasts(type);
CREATE INDEX podLastPubDate_idx ON podcasts(latest_publication_date);
CREATE INDEX podCustomName_idx ON podcasts(custom_name COLLATE NOCASE);
CREATE INDEX podPriority_idx ON podcasts(priority);
CREATE INDEX podIsVirtual_idx ON podcasts(is_virtual);
CREATE INDEX podUpdateDate_idx ON podcasts(update_date);
CREATE INDEX podLanguage_idx ON podcasts(language);
CREATE INDEX podThumbnailId_idx ON podcasts(thumbnail_id);
CREATE INDEX podFeedUrl_idx ON podcasts(feed_url);
CREATE INDEX podAutoRefresh_idx ON podcasts(automaticRefresh);
CREATE INDEX podNotifMuted_idx ON podcasts(subscribed_status, _id, muted);
CREATE INDEX commentsPodId_idx ON comments(podcast_id);
CREATE INDEX commentsPodIdNewStatus_idx ON comments(podcast_id, new_status);
CREATE INDEX commentsNewStatusEpId_idx ON comments(new_status, episode_id);
CREATE INDEX commentsEpIdGuid_idx ON comments(episode_id, guid);
CREATE INDEX commentsIdGuid_idx ON comments(_id, guid);
CREATE INDEX teamsName_idx ON teams(name);
CREATE INDEX teamsLanguage_idx ON teams(language);
CREATE INDEX teamsThumbnailId_idx ON teams(thumbnail_id);
CREATE INDEX timestampList_idx ON timestamp_list(timestamp, type);
CREATE INDEX serverActType_idx ON server_action(type);
CREATE INDEX serverActTypeEntId_idx ON server_action(type, entityId);
CREATE INDEX tagsName_idx ON tags(name);
CREATE INDEX tagRelationTagId_idx ON tag_relation(tag_id);
CREATE INDEX tagRelationPodId_idx ON tag_relation(podcast_id);
CREATE INDEX statEntityType_idx ON statistics(entityType);
CREATE INDEX statType_idx ON statistics(type);
CREATE INDEX bitmapUrl_idx ON bitmaps(url);
CREATE INDEX supportedLangShortName_idx ON supported_languages(short_name);
CREATE INDEX supportedLangLongName_idx ON supported_languages(long_name);
CREATE INDEX radioSearchResultsType_idx ON radio_search_results (type);
CREATE INDEX radioSearchResultsName_idx ON radio_search_results (name, type);
CREATE INDEX radioSearchResultsCountry_idx ON radio_search_results (country, type);
CREATE INDEX radioSearchResultsThumbnail_idx ON radio_search_results (thumbnail_id);
CREATE INDEX chaptersPodId_idx ON chapters (podcastId);
CREATE INDEX chaptersEpIdBook_idx ON chapters (episodeId, customBookmark);
CREATE INDEX popSearchTermsLang_idx ON popular_search_terms(languages);
CREATE INDEX genresName_idx ON genres(name);
CREATE INDEX genreRelationTagId_idx ON genre_relation(genre_id);
CREATE INDEX genreRelationPodId_idx ON genre_relation(radio_id);
CREATE INDEX podRelatedId_idx ON relatedPodcasts(url, relation);
CREATE INDEX reviewsPodId_idx ON reviews(podcast_id);
CREATE INDEX reviewsMyRev_idx ON reviews(isMyReview);
CREATE INDEX reviewsServerId_idx ON reviews(serverId);
CREATE INDEX adCampLangCateg_idx ON ad_campaign(language, enabled, category_id);
CREATE INDEX adCampCateg_idx ON ad_campaign(enabled, category_id);
CREATE INDEX adCampServId_idx ON ad_campaign(server_id);
CREATE INDEX adCuratedList1_idx ON curated_lists(server_id, enabled);
CREATE INDEX adCuratedList2_idx ON curated_lists(server_id);
CREATE INDEX adCuratedList3_idx ON curated_lists(enabled);
CREATE INDEX adCuratedList4_idx ON curated_lists(header_id);
CREATE INDEX adCuratedList5_idx ON curated_lists(banner_id);
CREATE INDEX iHAMain_idx ON iha(enabled, format);
CREATE INDEX iHAServId_idx ON iha(server_id);
CREATE INDEX iHABmp1_idx ON iha(artwork_portrait_id);
CREATE INDEX iHABmp2_idx ON iha(artwork_landscape_id);
CREATE TABLE sqlite_stat1(tbl, idx, stat);
CREATE INDEX episodesPlayBackHistory_idx ON episodes(
    playbackDate,
    position_to_resume,
    seen_status,
    podcast_id
);
CREATE INDEX episodesPlayBackInProgress_idx ON episodes(
    position_to_resume,
    duration_ms,
    seen_status,
    podcast_id
);
CREATE INDEX bitmapLocalFile_idx ON bitmaps(local_file, is_downloaded);
CREATE INDEX bitmapIsDownloaded_idx ON bitmaps(is_downloaded, md5);
CREATE INDEX chaptersCoontent_idx ON chapters (name, description);
CREATE TABLE persons (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    picture_id INTEGER NOT NULL DEFAULT -1,
    bio_url TEXT,
    UNIQUE(name, picture_id, bio_url) ON CONFLICT REPLACE
);
CREATE INDEX personName_idx ON persons(name);
CREATE TABLE person_relation (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER NOT NULL DEFAULT -1,
    podcast_id INTEGER NOT NULL DEFAULT -1,
    episode_id INTEGER NOT NULL DEFAULT -1,
    role TEXT,
    category TEXT
);
CREATE INDEX perRelPerId_idx ON person_relation(person_id);
CREATE INDEX perRelPodId_idx ON person_relation(podcast_id);
CREATE INDEX perRelEpId_idx ON person_relation(episode_id);
CREATE INDEX perRelEpPodId_idx ON person_relation(podcast_id, episode_id);
CREATE INDEX chaptersArtworkId_idx ON chapters (artworkId);
CREATE TABLE locations (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    data TEXT,
    UNIQUE(name, data) ON CONFLICT REPLACE
);
CREATE INDEX locationName_idx ON locations(name);
CREATE TABLE location_relation (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    location_id INTEGER NOT NULL DEFAULT -1,
    podcast_id INTEGER NOT NULL DEFAULT -1,
    episode_id INTEGER NOT NULL DEFAULT -1
);
CREATE INDEX locRelLocId_idx ON location_relation(location_id);
CREATE INDEX locRelPodId_idx ON location_relation(podcast_id);
CREATE INDEX locRelEpId_idx ON location_relation(episode_id);
CREATE INDEX locRelEpPodId_idx ON location_relation(podcast_id, episode_id);
CREATE TABLE episode_search_results (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    search_type INTEGER NOT NULL,
    category_id INTEGER DEFAULT -1,
    podcastId INTEGER DEFAULT -1,
    podcastServerId INTEGER NOT NULL,
    podcastFeedUrl TEXT NOT NULL collate nocase,
    podcastName TEXT NOT NULL,
    author TEXT,
    language TEXT NOT NULL,
    podcast_thumbnail_id INTEGER DEFAULT -1,
    iTunesId TEXT,
    episodeId INTEGER DEFAULT -1,
    episodeServerId INTEGER NOT NULL,
    episodeUrl TEXT NOT NULL collate nocase,
    episodeName TEXT NOT NULL,
    description TEXT NOT NULL,
    episode_thumbnail_id INTEGER DEFAULT -1,
    publicationDate INTEGER NOT NULL,
    duration INTEGER DEFAULT -1,
    type TEXT NOT NULL,
    score INTEGER NOT NULL,
    UNIQUE (search_type, category_id, episodeUrl)
);
CREATE TABLE topics (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL collate nocase,
    keywords TEXT NOT NULL,
    nbDays INTEGER NOT NULL DEFAULT 3,
    position INTEGER NOT NULL DEFAULT 1,
    UNIQUE (name)
);
CREATE INDEX topicsPos_idx ON topics(position);
CREATE INDEX episodesDownloadedUrl_idx ON episodes(download_url);
CREATE INDEX orderedListMain_idx ON ordered_list(type, filter, id);
CREATE INDEX orderedListSecond_idx ON ordered_list(type, id);
CREATE TABLE alarms (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    time INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    frequency INTEGER NOT NULL DEFAULT 0,
    name TEXT collate nocase,
    type INTEGER NOT NULL DEFAULT 0,
    entityId INTEGER NOT NULL DEFAULT 0,
    volume INTEGER NOT NULL DEFAULT 5
);
CREATE INDEX alarmsEnabled_idx ON alarms(enabled);
CREATE TABLE social (
    _id INTEGER PRIMARY KEY AUTOINCREMENT,
    podcastId INTEGER NOT NULL,
    episodeId INTEGER NOT NULL,
    priority INTEGER NOT NULL,
    platform TEXT NOT NULL,
    accountId TEXT NOT NULL,
    url TEXT NOT NULL,
    date INTEGER DEFAULT -1,
    json TEXT
);
CREATE INDEX socialPodId_idx ON social (podcastId, priority);
CREATE INDEX socialEpId_idx ON social (episodeId, priority);
CREATE INDEX chaptersBookmark_idx ON chapters(customBookmark);
CREATE INDEX podNextEopisodeDate_idx ON podcasts(subscribed_status, next_episode_forecast_date);
CREATE INDEX episodesGUIDs_idx ON episodes(guid, podcast_id);
CREATE INDEX episodesThumbsRating_idx ON episodes(thumbsRating);