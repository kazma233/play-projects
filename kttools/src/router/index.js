import { createRouter, createWebHistory } from 'vue-router';
import { toolMetaMap } from '../constants/tool-meta'

const routes = [
    { path: '/', redirect: '/datetime' },
    { path: '/base64', name: 'Base64Tools', component: () => import('../components/Base64Tools.vue'), meta: toolMetaMap['/base64'] },
    { path: '/url', name: 'URLTools', component: () => import('../components/URLTools.vue'), meta: toolMetaMap['/url'] },
    { path: '/datetime', name: 'DateTimeTools', component: () => import('../components/DateTimeTools.vue'), meta: toolMetaMap['/datetime'] },
    { path: '/json', name: 'JsonTools', component: () => import('../components/JsonTools.vue'), meta: toolMetaMap['/json'] },
    { path: '/sha1', name: 'Sha1Tools', component: () => import('../components/Sha1Tools.vue'), meta: toolMetaMap['/sha1'] },
    { path: '/md5', name: 'MD5Tools', component: () => import('../components/MD5Tools.vue'), meta: toolMetaMap['/md5'] },
    { path: '/qrcode', name: 'QrCode', component: () => import('../views/QrCode.vue'), meta: toolMetaMap['/qrcode'] },
    { path: '/ports', name: 'PortsTools', component: () => import('../components/PortsTools.vue'), meta: toolMetaMap['/ports'] },
    { path: '/image', name: 'ImageTools', component: () => import('../components/ImageTools.vue'), meta: toolMetaMap['/image'] },

];

const router = createRouter({
    history: createWebHistory(),
    routes
});

export default router;
