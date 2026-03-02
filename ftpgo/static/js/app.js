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
        const fileInput = ref(null);
        const folderInput = ref(null);

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
                let comparison = 0;
                switch (sortField.value) {
                    case 'name':
                        comparison = a.name.localeCompare(b.name, 'zh-CN');
                        break;
                    case 'size':
                        comparison = (a.isDir ? -1 : (a.size || 0)) - (b.isDir ? -1 : (b.size || 0));
                        break;
                    case 'modTime':
                        comparison = new Date(a.modTime) - new Date(b.modTime);
                        break;
                }

                if (sortDirection.value === 'desc') comparison = -comparison;

                if (comparison === 0 && a.isDir !== b.isDir) {
                    return sortDirection.value === 'asc' ? (a.isDir ? -1 : 1) : (a.isDir ? 1 : -1);
                }
                return comparison;
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

        const deleteSelected = async () => {
            if (!selectedFiles.value.length || !window.confirm(`确定要删除选中的 ${selectedFiles.value.length} 项吗？`)) return;

            let success = 0, fail = 0;
            for (const path of selectedFiles.value) {
                (await deleteFile(path, false)) ? success++ : fail++;
            }
            showMessage(fail ? `删除完成：成功 ${success} 项，失败 ${fail} 项` : `成功删除 ${success} 项`, fail ? 'error' : 'success');
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
                const res = await fetch(`/api/download?path=${encodeURIComponent(file.path)}`);
                if (!res.ok) {
                    previewContent.value = '无法加载文件内容';
                    return;
                }
                const blob = await res.blob();
                const MAX = 128 * 1024;
                isPreviewTruncated.value = blob.size > MAX;
                previewContent.value = await (isPreviewTruncated.value ? blob.slice(0, MAX) : blob).text();
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

            if (errors.length > 0 && uploaded === 0) {
                showMessage(`${actionName}失败: ${errors.join(', ')}`, 'error');
            } else if (errors.length > 0) {
                showMessage(`${actionName}成功: ${uploaded} 个文件，失败: ${errors.join(', ')}`, 'error');
                loadFiles();
            } else {
                showMessage(`${actionName}成功: ${uploaded} 个文件`);
                loadFiles();
            }
            setTimeout(() => uploadProgress.value = 0, 500);
        };

        const uploadFiles = async (fileList, onProgress) => {
            if (!fileList?.length) return { uploaded: 0, errors: [] };

            isUploading.value = true;
            uploadStatus.value = `准备上传 ${fileList.length} 个文件...`;

            const BATCH_SIZE = 50;
            let uploadedCount = 0;
            let errorMessages = [];

            const totalBatches = Math.ceil(fileList.length / BATCH_SIZE);
            for (let i = 0; i < fileList.length; i += BATCH_SIZE) {
                const batch = fileList.slice(i, i + BATCH_SIZE);
                const formData = new FormData();
                const currentBatch = Math.floor(i / BATCH_SIZE) + 1;

                uploadStatus.value = `上传中 ${currentBatch}/${totalBatches}...`;

                for (const file of batch) {
                    formData.append('files', file);
                    formData.append('relativePaths', file.relativePath || file.name);
                }

                try {
                    const res = await fetch(`/api/upload?path=${encodeURIComponent(currentPath.value)}`, {
                        method: 'POST',
                        body: formData
                    });
                    const data = await res.json();

                    if (data.uploaded) uploadedCount += data.uploaded.length;
                    if (data.errors) errorMessages.push(...data.errors);
                    if (onProgress) onProgress(Math.min((uploadedCount / fileList.length) * 100, 100));
                } catch (err) {
                    const batchNames = batch.slice(0, 3).map(f => f.name).join(', ');
                    const more = batch.length > 3 ? ` (+${batch.length - 3} more)` : '';
                    errorMessages.push(`批次 ${Math.floor(i / BATCH_SIZE) + 1} (${batchNames}${more}): ${err.message}`);
                }
            }

            isUploading.value = false;
            return { uploaded: uploadedCount, errors: errorMessages };
        };

        const handleFileUpload = async (e) => {
            uploadProgress.value = 0;
            const files = [...e.target.files];
            const result = await uploadFiles(files, (p) => uploadProgress.value = p);
            handleUploadResult(result);
            e.target.value = '';
        };

        const handleFolderUpload = async (e) => {
            const files = [...e.target.files].map(f => (f.relativePath = f.webkitRelativePath || f.name, f));
            uploadProgress.value = 0;
            const result = await uploadFiles(files, (p) => uploadProgress.value = p);
            handleUploadResult(result);
            e.target.value = '';
        };

        const traverseFileTree = async (item, path, result) => {
            if (item.isFile) {
                result.push(await new Promise(resolve => item.file(f => resolve((f.relativePath = path + f.name, f)))));
            } else if (item.isDirectory) {
                const reader = item.createReader();
                const read = async () => {
                    const entries = await new Promise(resolve => reader.readEntries(resolve));
                    if (entries.length) {
                        await Promise.all(entries.map(e => traverseFileTree(e, `${path}${item.name}/`, result)));
                        await read();
                    }
                };
                await read();
            }
        };

        const handleDrop = async (e) => {
            isDragging.value = false;
            const result = [];
            const items = e.dataTransfer.items;

            if (items?.length > 0 && typeof items[0].webkitGetAsEntry === 'function') {
                const entries = [];
                for (let i = 0; i < items.length; i++) {
                    const entry = items[i].webkitGetAsEntry();
                    if (entry) entries.push(entry);
                }
                await Promise.all(entries.map(entry => traverseFileTree(entry, '', result)));
            } else {
                for (const file of e.dataTransfer.files) {
                    file.relativePath = file.webkitRelativePath || file.name;
                    result.push(file);
                }
            }

            if (result.length > 0) {
                uploadProgress.value = 0;
                const uploadResult = await uploadFiles(result, (p) => uploadProgress.value = p);
                handleUploadResult(uploadResult);
            }
        };

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
            fileInput, folderInput, pathSegments, isAllSelected, previewUrl, isImage, isText,
            sortField, sortDirection, sortedFiles,
            navigateTo, navigateUp, navigateToSegment, toggleSelectAll, toggleSort,
            createFolder, deleteFile, deleteSelected, showRenameModal, doRename,
            downloadFile, downloadSelected, previewFile,
            handleFileUpload, handleFolderUpload, handleDrop,
            getFileIcon, formatSize, formatDate
        };
    }
}).mount('#app');
