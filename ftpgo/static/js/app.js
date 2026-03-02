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
        const message = ref('');
        const messageType = ref('success');
        
        // 排序状态（默认按修改时间降序）
        const sortField = ref('modTime');
        const sortDirection = ref('desc');
        
        // 弹窗状态
        const showNewFolderModal = ref(false);
        const showRenameDialog = ref(false);
        const showPreview = ref(false);
        
        // 表单数据
        const newFolderName = ref('');
        const renameOldPath = ref('');
        const renameNewName = ref('');
        
        // 预览状态
        const previewPath = ref('');
        const previewFileName = ref('');
        const previewContent = ref('');
        const isPreviewTruncated = ref(false);
        
        // DOM 引用
        const fileInput = ref(null);
        const folderInput = ref(null);

        // 文本文件扩展名（按字母排序，便于维护）
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
        
        // 排序后的文件列表
        const sortedFiles = computed(() => {
            const result = [...files.value];
            return result.sort((a, b) => {
                // 先按字段排序
                let comparison = 0;
                switch (sortField.value) {
                    case 'name':
                        comparison = a.name.localeCompare(b.name, 'zh-CN');
                        break;
                    case 'size':
                        // 文件夹参与排序，用 -1 表示（升序时会排在前面）
                        const aSize = a.isDir ? -1 : (a.size || 0);
                        const bSize = b.isDir ? -1 : (b.size || 0);
                        comparison = aSize - bSize;
                        break;
                    case 'modTime':
                        comparison = new Date(a.modTime) - new Date(b.modTime);
                        break;
                }
                
                // 应用排序方向
                if (sortDirection.value === 'desc') {
                    comparison = -comparison;
                }
                
                // 如果字段值相等，根据排序方向决定文件夹位置
                // 升序：文件夹在前；降序：文件夹在后
                if (comparison === 0 && a.isDir !== b.isDir) {
                    if (sortDirection.value === 'asc') {
                        return a.isDir ? -1 : 1;
                    } else {
                        return a.isDir ? 1 : -1;
                    }
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

        const getFileIcon = (file) => {
            if (file.isDir) return 'fas fa-folder text-yellow-500';
            const ext = getExt(file.name);
            const icons = {
                jpg: 'fas fa-image text-purple-500', jpeg: 'fas fa-image text-purple-500', png: 'fas fa-image text-purple-500', gif: 'fas fa-image text-purple-500',
                mp4: 'fas fa-video text-red-500', mp3: 'fas fa-music text-blue-500', pdf: 'fas fa-file-pdf text-red-600',
                doc: 'fas fa-file-word text-blue-600', docx: 'fas fa-file-word text-blue-600', xls: 'fas fa-file-excel text-green-600', xlsx: 'fas fa-file-excel text-green-600',
                zip: 'fas fa-file-archive text-yellow-600', txt: 'fas fa-file-alt text-gray-500'
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
            try {
                const res = await fetch(`/api/browse?path=${encodeURIComponent(currentPath.value)}`);
                const data = await res.json();
                files.value = data.files || [];
                selectedFiles.value = [];
            } catch (err) {
                showMessage('加载文件失败: ' + err.message, 'error');
            }
        };

        const apiRequest = async (url, body, successMsg) => {
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
            }
            return false;
        };

        // 导航操作
        const navigateTo = (path) => {
            currentPath.value = path;
            loadFiles();
        };

        const navigateUp = () => navigateTo(currentPath.value.split('/').filter(Boolean).slice(0, -1).join('/'));
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

        // 选择操作
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
            if (!selectedFiles.value.length) return;
            if (!window.confirm(`确定要删除选中的 ${selectedFiles.value.length} 项吗？`)) return;
            
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
            }
        };

        // 上传
        const uploadFiles = async (fileList) => {
            if (!fileList?.length) return;
            
            const formData = new FormData();
            for (const file of fileList) {
                formData.append('files', file);
                formData.append('relativePaths', file.relativePath || file.name);
            }

            try {
                uploadProgress.value = 0;
                const res = await fetch(`/api/upload?path=${encodeURIComponent(currentPath.value)}`, {
                    method: 'POST',
                    body: formData
                });
                const data = await res.json();
                res.ok ? showMessage(`上传成功: ${data.uploaded.length} 个文件`) : showMessage(data.error || '上传失败', 'error');
                if (res.ok) loadFiles();
            } catch (err) {
                showMessage('上传失败: ' + err.message, 'error');
            } finally {
                uploadProgress.value = 100;
                setTimeout(() => uploadProgress.value = 0, 500);
            }
        };

        const handleFileUpload = (e) => {
            uploadFiles(e.target.files);
            e.target.value = '';
        };

        const handleFolderUpload = (e) => {
            const files = [...e.target.files].map(f => (f.relativePath = f.webkitRelativePath || f.name, f));
            uploadFiles(files);
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
            
            if (items?.[0] && typeof items[0].webkitGetAsEntry === 'function') {
                for (let i = 0; i < items.length; i++) {
                    const entry = items[i].webkitGetAsEntry();
                    if (entry) await traverseFileTree(entry, '', result);
                }
            } else {
                for (const file of e.dataTransfer.files) {
                    file.relativePath = file.webkitRelativePath || file.name;
                    result.push(file);
                }
            }
            await uploadFiles(result);
        };

        // 剪贴板
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

        const handlePaste = async (e) => {
            const data = e.clipboardData || window.clipboardData;
            if (!data) return;

            // 优先处理文件
            if (data.files.length) {
                const files = await Promise.all([...data.files].map(async f => {
                    const name = await generateFilename(getExt(f.name) || 'bin', f);
                    return Object.assign(new File([f], name, { type: f.type }), { relativePath: name });
                }));
                await uploadFiles(files);
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
                await uploadFiles(images);
                return;
            }

            // 处理文本
            const text = data.getData('text/plain');
            if (text?.trim()) {
                const name = await generateFilename('txt', text);
                const file = Object.assign(new File([text], name, { type: 'text/plain' }), { relativePath: name });
                await uploadFiles([file]);
                showMessage(`已创建文本文件: ${name}`);
            }
        };

        onMounted(() => {
            loadFiles();
            document.addEventListener('paste', handlePaste);
        });
        onUnmounted(() => document.removeEventListener('paste', handlePaste));

        return {
            files, currentPath, selectedFiles, isDragging, uploadProgress, message, messageType,
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
