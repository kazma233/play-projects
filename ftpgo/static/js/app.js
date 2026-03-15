// 文件管理器 Vue 3 应用

const { createApp, ref, computed, onMounted, onUnmounted } = Vue;

createApp({
    setup() {
        // 响应式状态
        const files = ref([]);
        const currentPath = ref('');
        const selectedFiles = ref([]);
        const isDragging = ref(false);
        const uploadProgress = ref(0);
        const isUploading = ref(false);
        const uploadStatus = ref('');
        const isLoading = ref(false);
        const loadingText = ref('加载中...');
        const loadingCount = ref(0);
        const message = ref('');
        const messageType = ref('success');
        const sortField = ref('modTime');
        const sortDirection = ref('desc');
        const showNewFolderModal = ref(false);
        const showRenameDialog = ref(false);
        const showPreview = ref(false);
        const newFolderName = ref('');
        const renameOldPath = ref('');
        const renameNewName = ref('');
        const previewPath = ref('');
        const previewFileName = ref('');
        const previewContent = ref('');
        const isPreviewTruncated = ref(false);
        const pasteUploadEnabled = ref(false);

        const togglePasteUpload = () => {
            pasteUploadEnabled.value = !pasteUploadEnabled.value;
        };

        const textExtensions = [
            'bash', 'bat', 'c', 'cfg', 'cmd', 'cmake', 'conf', 'cpp', 'cs', 'css', 'dockerfile', 'dockerignore',
            'env', 'gitignore', 'go', 'h', 'hpp', 'html', 'htm', 'ini', 'java', 'js', 'json', 'jsx', 'less',
            'log', 'makefile', 'markdown', 'md', 'php', 'properties', 'ps1', 'py', 'rb', 'rs', 'sass', 'scss',
            'sh', 'sql', 'toml', 'ts', 'tsx', 'txt', 'xml', 'yaml', 'yml', 'zsh'
        ];

        // 计算属性
        const pathSegments = computed(() => currentPath.value ? currentPath.value.split('/').filter(Boolean) : []);
        const isAllSelected = computed(() => files.value.length > 0 && selectedFiles.value.length === files.value.length);
        const previewUrl = computed(() => `/api/download?path=${encodeURIComponent(previewPath.value)}`);
        const isImage = computed(() => ['jpg', 'jpeg', 'png', 'gif', 'webp', 'bmp'].includes(getExt(previewFileName.value)));
        const isText = computed(() => textExtensions.includes(getExt(previewFileName.value)));

        const sortedFiles = computed(() => {
            const result = [...files.value];
            return result.sort((a, b) => {
                // 目录始终排在前面，保持稳定性
                if (a.isDir !== b.isDir) {
                    return a.isDir ? -1 : 1;
                }

                let comparison = 0;
                switch (sortField.value) {
                    case 'name':
                        // 使用预创建的 Collator 提高性能
                        comparison = collator.compare(a.name, b.name);
                        break;
                    case 'size':
                        comparison = (a.size || 0) - (b.size || 0);
                        break;
                    case 'modTime':
                        comparison = new Date(a.modTime) - new Date(b.modTime);
                        break;
                }

                return sortDirection.value === 'desc' ? -comparison : comparison;
            });
        });

        // 工具函数
        const getExt = (filename) => filename.split('.').pop().toLowerCase();

        const showMessage = (msg, type = 'success') => {
            message.value = msg;
            messageType.value = type;
            setTimeout(() => message.value = '', 3000);
        };

        // Loading 控制函数 - 使用计数器处理并发请求
        const showLoading = (text = '加载中...') => {
            loadingCount.value++;
            loadingText.value = text;
            isLoading.value = true;
        };

        const hideLoading = () => {
            loadingCount.value = Math.max(0, loadingCount.value - 1);
            if (loadingCount.value === 0) {
                isLoading.value = false;
            }
        };

        const getFileIcon = (file) => {
            if (file.isDir) return 'fas fa-folder text-yellow-500';
            const ext = getExt(file.name);
            const icons = {
                jpg: 'fas fa-image text-purple-500', jpeg: 'fas fa-image text-purple-500', png: 'fas fa-image text-purple-500', gif: 'fas fa-image text-purple-500',
                mp4: 'fas fa-video text-red-500', mp3: 'fas fa-music text-blue-500', pdf: 'fas fa-file-pdf text-red-600',
                doc: 'fas fa-file-word text-blue-600', docx: 'fas fa-file-word text-blue-600', xls: 'fas fa-file-excel text-green-600', xlsx: 'fas fa-file-excel text-green-600',
                zip: 'fas fa-file-archive text-yellow-600', txt: 'fas fa-file-lines text-gray-500'
            };
            return icons[ext] || 'fas fa-file text-gray-400';
        };

        const formatSize = (bytes) => {
            if (!bytes) return '0 B';
            const units = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log2(bytes) / 10);
            return (bytes / (1 << (i * 10))).toFixed(2) + ' ' + units[i];
        };

        const formatDate = (dateStr) => new Date(dateStr).toLocaleString('zh-CN');

        // API 操作
        const loadFiles = async () => {
            showLoading('加载文件列表...');
            try {
                const res = await fetch(`/api/browse?path=${encodeURIComponent(currentPath.value)}`);
                if (!res.ok) {
                    let errorMessage = '加载文件失败';
                    try {
                        const errorData = await res.json();
                        if (errorData?.error) {
                            errorMessage = errorData.error;
                        }
                    } catch (_e) {
                    }
                    throw new Error(errorMessage);
                }
                const data = await res.json();
                files.value = data.files || [];
                selectedFiles.value = [];
            } catch (err) {
                showMessage('加载文件失败: ' + err.message, 'error');
            } finally {
                hideLoading();
            }
        };

        const apiRequest = async (url, body, successMsg) => {
            showLoading('处理中...');
            try {
                const res = await fetch(url, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                if (res.ok) {
                    showMessage(successMsg);
                    loadFiles();
                    return true;
                }
                const data = await res.json();
                showMessage(data.error || '操作失败', 'error');
            } catch (err) {
                showMessage('操作失败: ' + err.message, 'error');
            } finally {
                hideLoading();
            }
            return false;
        };

        // 导航操作
        const navigateTo = (path) => {
            currentPath.value = path;
            loadFiles();
        };

        const navigateUp = () => navigateTo(pathSegments.value.slice(0, -1).join('/'));
        const navigateToSegment = (index) => navigateTo(pathSegments.value.slice(0, index + 1).join('/'));

        // 排序操作
        const toggleSort = (field) => {
            if (sortField.value === field) {
                sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc';
            } else {
                sortField.value = field;
                sortDirection.value = 'asc';
            }
        };

        const toggleSelectAll = () => {
            selectedFiles.value = isAllSelected.value ? [] : files.value.map(f => f.path);
        };

        // 文件操作
        const createFolder = () => {
            if (!newFolderName.value.trim()) return;
            apiRequest('/api/mkdir', { path: currentPath.value + '/' + newFolderName.value }, '文件夹创建成功');
            showNewFolderModal.value = false;
            newFolderName.value = '';
        };

        const deleteFile = async (path, confirm = true) => {
            if (confirm && !window.confirm('确定要删除吗？')) return;
            return apiRequest('/api/delete', { path }, '删除成功');
        };

        // 批量删除 - 并发处理，提高速度
        const deleteSelected = async () => {
            if (!selectedFiles.value.length || !window.confirm(`确定要删除选中的 ${selectedFiles.value.length} 项吗？`)) return;

            const CONCURRENT_LIMIT = 5; // 同时最多5个并发删除
            const queue = [...selectedFiles.value];
            let success = 0, fail = 0;

            // 创建并发工作器
            const workers = Array(CONCURRENT_LIMIT).fill().map(async () => {
                while (queue.length > 0) {
                    const path = queue.shift();
                    try {
                        const res = await fetch('/api/delete', {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({ path })
                        });
                        if (res.ok) {
                            success++;
                        } else {
                            fail++;
                        }
                    } catch (e) {
                        fail++;
                    }
                }
            });

            showLoading(`正在删除 ${selectedFiles.value.length} 个文件...`);
            
            await Promise.all(workers);
            
            hideLoading();
            showMessage(
                fail ? `删除完成：成功 ${success} 项，失败 ${fail} 项` : `成功删除 ${success} 项`,
                fail ? 'error' : 'success'
            );
            
            selectedFiles.value = [];
            loadFiles();
        };

        const showRenameModal = (file) => {
            renameOldPath.value = file.path;
            renameNewName.value = file.name;
            showRenameDialog.value = true;
        };

        const doRename = async () => {
            if (!renameNewName.value.trim()) return;
            const dir = renameOldPath.value.substring(0, renameOldPath.value.lastIndexOf('/'));
            if (await apiRequest('/api/rename', { oldPath: renameOldPath.value, newPath: dir + '/' + renameNewName.value }, '重命名成功')) {
                showRenameDialog.value = false;
            }
        };

        const downloadFile = (path) => window.open(`/api/download?path=${encodeURIComponent(path)}`, '_blank');

        const downloadSelected = () => {
            if (selectedFiles.value.length === 1) {
                downloadFile(selectedFiles.value[0]);
            } else {
                window.open(`/api/download-zip?paths=${encodeURIComponent(JSON.stringify(selectedFiles.value))}`, '_blank');
            }
        };

        // 预览
        const previewFile = async (file) => {
            previewPath.value = file.path;
            previewFileName.value = file.name;
            previewContent.value = '';
            isPreviewTruncated.value = false;
            showPreview.value = true;
            if (textExtensions.includes(getExt(file.name))) await loadTextContent(file);
        };

        const loadTextContent = async (file) => {
            showLoading('加载文件内容...');
            try {
                const MAX_PREVIEW_SIZE = 128 * 1024; // 128KB
                
                // 使用 Range 请求只获取前128KB，减少网络和内存开销
                const res = await fetch(`/api/download?path=${encodeURIComponent(file.path)}&inline=true`, {
                    headers: {
                        'Range': `bytes=0-${MAX_PREVIEW_SIZE - 1}`
                    }
                });
                
                if (res.status === 206) {
                    // 服务器支持 Range 请求 (Partial Content)
                    const contentRange = res.headers.get('Content-Range');
                    const totalMatch = contentRange?.match(/\/([0-9]+)$/);
                    const totalSize = totalMatch ? parseInt(totalMatch[1]) : 0;
                    isPreviewTruncated.value = totalSize > MAX_PREVIEW_SIZE;
                    
                    const blob = await res.blob();
                    previewContent.value = await blob.text();
                } else if (res.ok) {
                    // 服务器不支持 Range，回退到原有逻辑
                    const blob = await res.blob();
                    isPreviewTruncated.value = blob.size > MAX_PREVIEW_SIZE;
                    previewContent.value = await (isPreviewTruncated.value 
                        ? blob.slice(0, MAX_PREVIEW_SIZE) 
                        : blob
                    ).text();
                } else {
                    previewContent.value = '无法加载文件内容';
                }
            } catch (err) {
                previewContent.value = '加载文件内容失败: ' + err.message;
            } finally {
                hideLoading();
            }
        };

        // 上传处理 - 提取公共的上传结果处理函数
        const handleUploadResult = (result, actionName = '上传') => {
            uploadProgress.value = 100;
            const { uploaded, errors } = result;
            const total = uploaded + errors.length;

            if (errors.length > 0 && uploaded === 0) {
                showMessage(`${actionName}失败: 共 ${total} 个文件，全部失败`, 'error');
            } else if (errors.length > 0) {
                showMessage(`${actionName}完成: 共 ${total} 个文件，成功 ${uploaded} 个，失败 ${errors.length} 个`, 'error');
                loadFiles();
            } else {
                showMessage(`${actionName}成功: 共 ${uploaded} 个文件`);
                loadFiles();
            }
            setTimeout(() => uploadProgress.value = 0, 500);
        };

        const uploadFiles = async (fileList, onProgress) => {
            if (!fileList?.length) return { uploaded: 0, errors: [] };

            console.log(`[上传] 开始上传 ${fileList.length} 个文件`);
            
            isUploading.value = true;
            uploadStatus.value = `准备上传 ${fileList.length} 个文件...`;

            const BATCH_SIZE = 50;
            let uploadedCount = 0;
            let errorMessages = [];

            const totalBatches = Math.ceil(fileList.length / BATCH_SIZE);
            console.log(`[上传] 分 ${totalBatches} 批, 每批 ${BATCH_SIZE} 个`);
            
            for (let i = 0; i < fileList.length; i += BATCH_SIZE) {
                const batch = fileList.slice(i, i + BATCH_SIZE);
                const formData = new FormData();
                const currentBatch = Math.floor(i / BATCH_SIZE) + 1;
                
                console.log(`[上传] 批次 ${currentBatch}/${totalBatches}: ${batch.length} 个文件`);
                // 打印前3个文件名用于调试
                if (batch.length <= 5) {
                    console.log('[上传]   文件:', batch.map(f => f.relativePath || f.name).join(', '));
                } else {
                    console.log('[上传]   文件:', batch.slice(0, 3).map(f => f.relativePath || f.name).join(', '), `... +${batch.length - 3} more`);
                }

                uploadStatus.value = `上传中 ${currentBatch}/${totalBatches}...`;

                for (const file of batch) {
                    formData.append('files', file);
                    formData.append('relativePaths', file.relativePath || file.name);
                }

                try {
                    console.log(`[上传] 发送批次 ${currentBatch}...`);
                    const res = await fetch(`/api/upload?path=${encodeURIComponent(currentPath.value)}`, {
                        method: 'POST',
                        body: formData
                    });
                    const data = await res.json();
                    
                    console.log(`[上传] 批次 ${currentBatch} 响应:`, {
                        uploaded: data.uploaded?.length || 0,
                        errors: data.errors?.length || 0,
                        message: data.message
                    });
                    
                    if (data.uploaded) {
                        uploadedCount += data.uploaded.length;
                        console.log(`[上传] 批次 ${currentBatch} 成功: ${data.uploaded.length} 个`);
                    }
                    if (data.errors) {
                        errorMessages.push(...data.errors);
                        console.warn(`[上传] 批次 ${currentBatch} 错误:`, data.errors);
                    }
                    if (onProgress) onProgress(Math.min((uploadedCount / fileList.length) * 100, 100));
                } catch (err) {
                    console.error(`[上传] 批次 ${currentBatch} 请求失败:`, err);
                    // 整个批次失败（网络错误等），将该批次所有文件标记为失败
                    for (const file of batch) {
                        errorMessages.push(`${file.relativePath || file.name}: ${err.message}`);
                    }
                }
            }
            
            console.log(`[上传] 完成: 成功 ${uploadedCount}/${fileList.length}, 失败 ${errorMessages.length}`);

            isUploading.value = false;
            return { uploaded: uploadedCount, errors: errorMessages };
        };

        const handleFileUpload = async (e) => {
            const selectedFiles = [...e.target.files].map(f => {
                f.relativePath = f.name;
                return f;
            });
            if (selectedFiles.length === 0) return;

            uploadProgress.value = 0;
            const result = await uploadFiles(selectedFiles, (p) => uploadProgress.value = p);
            handleUploadResult(result);
            e.target.value = '';
        };

        const handleFolderUpload = async (e) => {
            const files = [...e.target.files].map(f => {
                f.relativePath = f.webkitRelativePath || f.name;
                return f;
            });
            
            if (files.length === 0) return;
            
            uploadProgress.value = 0;
            const result = await uploadFiles(files, (p) => uploadProgress.value = p);
            handleUploadResult(result);
            e.target.value = '';
        };

        // 优化：使用队列代替递归，避免栈溢出，并限制深度和文件数
        // 返回 { files: [], errors: [], totalFound: number }
        const traverseFileTree = async (item, path, globalCounter = { count: 0 }) => {
            const MAX_DEPTH = 10;       // 限制目录深度
            const MAX_FILES = 2000;     // 限制最大文件数
            const files = [];
            const errors = [];
            let localCount = 0;
            let dirReadCount = 0;
            
            // 队列存储待处理的项
            const queue = [{ item, path, depth: 0 }];
            // 使用 Map 跟踪每个目录的 reader，避免重复创建
            const dirReaders = new Map();
            
            console.log(`[遍历] 开始: ${item.name || 'root'}, 初始队列长度: ${queue.length}`);
            
            while (queue.length > 0) {
                // 检查全局计数器是否超过限制
                if (globalCounter.count >= MAX_FILES) {
                    console.warn(`[遍历] 已达到最大文件数限制 ${MAX_FILES}，停止遍历`);
                    break;
                }
                
                const { item: currentItem, path: currentPath, depth } = queue.shift();
                
                if (currentItem.isFile) {
                    try {
                        const file = await new Promise((resolve, reject) => {
                            currentItem.file(
                                f => resolve(Object.assign(f, { relativePath: currentPath + f.name })),
                                err => reject(err)
                            );
                        });
                        files.push(file);
                        globalCounter.count++;
                        localCount++;
                        if (localCount % 50 === 0) {
                            console.log(`[遍历] 已处理 ${localCount} 个文件, 队列: ${queue.length}`);
                        }
                    } catch (err) {
                        const filePath = currentPath + (currentItem.name || 'unknown');
                        errors.push({ path: filePath, error: err.message || '读取失败' });
                        console.warn(`[遍历] 读取文件失败: ${filePath}`, err);
                    }
                } else if (currentItem.isDirectory && depth < MAX_DEPTH) {
                    dirReadCount++;
                    try {
                        // 获取或创建 reader（每个目录只创建一个 reader）
                        let reader = dirReaders.get(currentItem);
                        let isNewReader = false;
                        if (!reader) {
                            reader = currentItem.createReader();
                            dirReaders.set(currentItem, reader);
                            isNewReader = true;
                        }
                        
                        const entries = await new Promise((resolve, reject) => {
                            reader.readEntries(resolve, reject);
                        });
                        
                        console.log(`[遍历] 目录读取 #${dirReadCount}: ${currentItem.name}, 新reader: ${isNewReader}, 条目数: ${entries.length}, 队列: ${queue.length}`);
                        
                        // 将子项加入队列（不使用递归）
                        let fileCount = 0;
                        let dirCount = 0;
                        for (const entry of entries) {
                            queue.push({
                                item: entry,
                                path: `${currentPath}${currentItem.name}/`,
                                depth: depth + 1
                            });
                            if (entry.isFile) fileCount++;
                            else if (entry.isDirectory) dirCount++;
                        }
                        console.log(`[遍历]   -> 添加 ${fileCount} 文件, ${dirCount} 目录到队列`);
                        
                        // 如果返回了 100 个条目，说明可能还有更多，将目录重新加入队列继续读取
                        if (entries.length === 100) {
                            queue.unshift({ 
                                item: currentItem, 
                                path: currentPath, 
                                depth 
                            });
                            console.log(`[遍历]   -> 目录未读完, 重新加入队列 (当前共${queue.length}项)`);
                        }
                    } catch (err) {
                        const dirPath = currentPath + (currentItem.name || 'unknown');
                        errors.push({ path: dirPath, error: err.message || '读取目录失败' });
                        console.warn(`[遍历] 读取目录失败: ${dirPath}`, err);
                    }
                }
                
                // 每处理50个文件让出主线程，避免阻塞UI
                if (localCount % 50 === 0) {
                    await new Promise(resolve => setTimeout(resolve, 0));
                }
            }
            
            console.log(`[遍历] 完成: ${item.name || 'root'}, 找到 ${files.length} 个文件, ${errors.length} 个错误, 目录读取次数: ${dirReadCount}`);
            return { files, errors, totalFound: globalCounter.count };
        };

        const handleDrop = async (e) => {
            isDragging.value = false;
            const files = [];
            const readErrors = [];
            const items = e.dataTransfer.items;
            
            console.log('[拖拽] 开始处理, items数量:', items?.length);

            if (items?.length > 0 && typeof items[0].webkitGetAsEntry === 'function') {
                const entries = [];
                for (let i = 0; i < items.length; i++) {
                    const entry = items[i].webkitGetAsEntry();
                    if (entry) entries.push(entry);
                    console.log(`[拖拽] item ${i}:`, entry?.name, entry?.isFile ? '文件' : '目录');
                }
                console.log('[拖拽] 有效entries:', entries.length);
                
                // 并行处理所有拖拽项
                const results = await Promise.all(entries.map(entry => traverseFileTree(entry, '')));
                for (const result of results) {
                    files.push(...result.files);
                    readErrors.push(...result.errors);
                }
            } else {
                console.log('[拖拽] 使用 fallback 模式');
                for (const file of e.dataTransfer.files) {
                    file.relativePath = file.webkitRelativePath || file.name;
                    files.push(file);
                }
            }
            
            console.log(`[拖拽] 总计: ${files.length} 个文件, ${readErrors.length} 个读取错误`);

            if (readErrors.length > 0) {
                console.warn('[拖拽] 读取失败的文件/目录:', readErrors);
            }

            if (files.length > 0 || readErrors.length > 0) {
                uploadProgress.value = 0;
                const uploadResult = await uploadFiles(files, (p) => uploadProgress.value = p);
                // 合并读取错误和上传错误
                const totalErrors = [
                    ...readErrors.map(e => `${e.path}: ${e.error}`),
                    ...uploadResult.errors
                ];
                console.log(`[拖拽] 上传完成: 成功 ${uploadResult.uploaded}, 失败 ${totalErrors.length}`);
                handleUploadResult({ 
                    uploaded: uploadResult.uploaded, 
                    errors: totalErrors 
                });
            }
        };

        // 预创建 Collator 提高中文排序性能
        const collator = new Intl.Collator('zh-CN', { sensitivity: 'base' });

        // 剪贴板处理 - 简化重复的上传逻辑
        const generateHash = async (data) => {
            const buffer = data instanceof Blob ? await data.arrayBuffer() : new TextEncoder().encode(data);
            const view = new Uint8Array(buffer);
            let hash = 5381;
            for (let i = 0; i < view.length; i++) hash = ((hash << 5) + hash) + view[i];
            return Math.abs(hash).toString(36).substring(0, 8).padEnd(8, '0');
        };

        const generateFilename = async (ext, content) => {
            const now = new Date();
            const date = `${now.getFullYear()}_${String(now.getMonth() + 1).padStart(2, '0')}_${String(now.getDate()).padStart(2, '0')}`;
            return `clipboard_${date}_${await generateHash(content)}.${ext}`;
        };

        const uploadFromClipboard = async (items, actionName) => {
            uploadProgress.value = 0;
            const result = await uploadFiles(items, (p) => uploadProgress.value = p);
            handleUploadResult(result, actionName);
        };

        const handlePaste = async (e) => {
            if (!pasteUploadEnabled.value) return;

            const data = e.clipboardData || window.clipboardData;
            if (!data) return;

            // 优先处理文件
            if (data.files.length) {
                const files = await Promise.all([...data.files].map(async f => {
                    const name = await generateFilename(getExt(f.name) || 'bin', f);
                    return Object.assign(new File([f], name, { type: f.type }), { relativePath: name });
                }));
                await uploadFromClipboard(files, '上传');
                return;
            }

            // 处理图片 items
            const images = [];
            for (const item of data.items) {
                if (item.type.includes('image')) {
                    const blob = item.getAsFile();
                    if (blob) {
                        const name = await generateFilename(item.type.split('/')[1] || 'png', blob);
                        images.push(Object.assign(new File([blob], name, { type: item.type }), { relativePath: name }));
                    }
                }
            }
            if (images.length) {
                await uploadFromClipboard(images, '上传');
                return;
            }

            // 处理文本
            const text = data.getData('text/plain');
            if (text?.trim()) {
                const name = await generateFilename('txt', text);
                const file = Object.assign(new File([text], name, { type: 'text/plain' }), { relativePath: name });
                await uploadFromClipboard([file], '创建');
            }
        };

        onMounted(() => {
            loadFiles();
            document.addEventListener('paste', handlePaste);
        });
        onUnmounted(() => document.removeEventListener('paste', handlePaste));

        return {
            files, currentPath, selectedFiles, isDragging, uploadProgress, isUploading, uploadStatus,
            isLoading, loadingText,
            message, messageType,
            showNewFolderModal, showRenameDialog, showPreview, newFolderName, renameNewName,
            previewPath, previewFileName, previewContent, isPreviewTruncated,
            pasteUploadEnabled, pathSegments, isAllSelected, previewUrl, isImage, isText,
            sortField, sortDirection, sortedFiles,
            navigateTo, navigateUp, navigateToSegment, toggleSelectAll, toggleSort, togglePasteUpload,
            createFolder, deleteFile, deleteSelected, showRenameModal, doRename,
            downloadFile, downloadSelected, previewFile,
            handleFileUpload, handleFolderUpload, handleDrop,
            getFileIcon, formatSize, formatDate
        };
    }
}).mount('#app');
